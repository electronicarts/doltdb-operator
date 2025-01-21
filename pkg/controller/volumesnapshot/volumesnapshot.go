package volumesnapshot

import (
	"context"
	"encoding/json"
	"fmt"
	cron "github.com/robfig/cron/v3"
	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	"github.com/electronicarts/doltdb-operator/pkg/builder"
	"gopkg.in/yaml.v3"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	klabels "k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type Reconciler struct {
	client.Client
	Builder *builder.Builder
}

// NewReconciler creates a new ServiceReconciler with the given client.
func NewReconciler(client client.Client, builder *builder.Builder) *Reconciler {
	return &Reconciler{
		Client:  client,
		Builder: builder,
	}
}

// ReconcileRequest contains the information needed to reconcile a CronJob.
type ReconcileRequest struct {
	Metadata *metav1.ObjectMeta
	Owner    *doltv1alpha.DoltDB
	SubOwner *doltv1alpha.Snapshot
}

// Reconcile ensures that the desired state of the CronJob is reflected in the cluster.
// If the CronJob does not exist, it will be created. If it exists, it will be patched with the new data.
func (r *Reconciler) Reconcile(ctx context.Context, req *ReconcileRequest) error {
	// Get all PVCs in the namespace with the same owner
	listOpts := &ctrlclient.ListOptions{
		LabelSelector: klabels.SelectorFromSet(
			builder.NewLabelsBuilder().
				WithDoltSelectorLabels(req.Owner).
				Build(),
		),
		Namespace: req.Owner.GetNamespace(),
	}
	// Get all PVCs in the namespace with the same owner
	pvcList := corev1.PersistentVolumeClaimList{}
	if err := r.List(ctx, &pvcList, listOpts); err != nil {
		return fmt.Errorf("error getting PVC for doltdb: %v", err)
	}

	// loop over pods and get pvcName
	for _, pvc := range pvcList.Items {
		// Create a VolumeSnapshot CR for each PVC
		pvcName := pvc.Name
		externalSnapshot, err := r.Builder.BuildExternalSnapshot(pvcName, req.Owner)
		if err != nil {
			return fmt.Errorf("error setting creating external snapshot builder: %v", err)
		}
		yamlData, err := jsonToYamlMarshal(externalSnapshot, err)
		if err != nil {
			return err
		}
		// Create or patch the ConfigMap for each PVC
		configMap, err := buildOrPatchConfigMap(ctx, req, yamlData, pvcName, err, r)
		if err != nil {
			return err
		}

		// Function to validate the FrequencySchedule
		_, err = cron.ParseStandard(*req.SubOwner.Spec.FrequencySchedule)
		if err != nil {
			return fmt.Errorf("invalid cron expression for FrequencySchedule: %v", err)
		}
		// Create or patch the CronJob for each PVC
		err = buildOrPatchCronJob(ctx, req, pvcName, configMap, err, r)
		if err != nil {
			return err
		}

	}
	return nil

}

func jsonToYamlMarshal(snapshot builder.VolumeSnapshot, err error) ([]byte, error) {
	// Convert the VolumeSnapshot to JSON first to remove omitempty fields then convert back to YAML.
	jsonData, _ := json.Marshal(&snapshot)
	var jsonObj map[string]interface{}
	err = json.Unmarshal([]byte(jsonData), &jsonObj)
	if err != nil {
		return nil, fmt.Errorf("error marshalling JSON for VolumeSnapshot: %v", err)
	}
	yamlData, err := yaml.Marshal(jsonObj)
	if err != nil {
		return nil, fmt.Errorf("error marshalling YAML for VolumeSnapshot: %v", err)
	}
	return yamlData, nil
}

// buildOrPatchConfigMap creates or patches a ConfigMap for the given PVC.
// If the ConfigMap does not exist, it will be created. If it exists, it will be patched with the new data.
func buildOrPatchConfigMap(ctx context.Context, req *ReconcileRequest, yamlData []byte, pvcName string, err error, r *Reconciler) (*corev1.ConfigMap, error) {
	// Create a ConfigMap for the VolumeSnapshot
	data := make(map[string]string)
	data[fmt.Sprintf("%s.yaml", req.Metadata.Name)] = string(yamlData)
	optsCM := builder.ConfigMapOpts{
		Metadata: req.Metadata,
		Key:      req.SubOwner.ConfigMapKey(pvcName),
		Data:     data,
	}
	configMap, err := r.Builder.BuildConfigMap(optsCM, req.Owner)
	if err != nil {
		return nil, fmt.Errorf("error building ConfigMap: %v", err)
	}

	var existingConfigMap corev1.ConfigMap
	// Check if the ConfigMap already exists and create or patch it accordingly
	if err := r.Get(ctx, optsCM.Key, &existingConfigMap); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("error getting ConfigMap: %v", err)
		}
		if err := r.Create(ctx, configMap); err != nil {
			return nil, fmt.Errorf("error creating ConfigMap: %v", err)
		}
	} else {
		patch := client.MergeFrom(existingConfigMap.DeepCopy())
		existingConfigMap.Data = configMap.Data
		err = r.Patch(ctx, &existingConfigMap, patch)

		if err != nil {
			return nil, fmt.Errorf("error patching ConfigMap: %v", err)
		}
	}
	return configMap, nil
}

// buildOrPatchCronJob creates or patches a CronJob for the given PVC.
// If the CronJob does not exist, it will be created. If it exists, it will be patched with the new data.
func buildOrPatchCronJob(ctx context.Context, req *ReconcileRequest, pvcName string, configMap *corev1.ConfigMap, err error, r *Reconciler) error {
	opts := builder.CronJobOpts{
		Metadata:      req.Metadata,
		Key:           req.SubOwner.CronJobKey(pvcName),
		ConfigMapName: configMap.Name,
		Schedule:      *req.SubOwner.Spec.FrequencySchedule,
	}
	// Create a CronJob for each PVC
	cronJob, err := r.Builder.BuildCronJob(opts, req.Owner, req.SubOwner)
	if err != nil {
		return fmt.Errorf("error building cronJob: %v", err)
	}
	var existingCronJob batchv1.CronJob
	// Check if the CronJob already exists and create or patch it accordingly
	if err := r.Get(ctx, opts.Key, &existingCronJob); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("error getting CronJob: %v", err)
		}
		if err := r.Create(ctx, cronJob); err != nil {
			return fmt.Errorf("error creating CronJob: %v", err)
		}
	} else {
		patchCron := client.MergeFrom(existingCronJob.DeepCopy())
		existingCronJob.Spec = cronJob.Spec
		err = r.Patch(ctx, &existingCronJob, patchCron)
		if err != nil {
			return fmt.Errorf("error patching CronJob: %v", err)
		}
	}
	return nil
}
