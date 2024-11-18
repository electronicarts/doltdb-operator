load('ext://git_resource', 'git_checkout')
load('ext://namespace', 'namespace_create', 'namespace_inject')
load('ext://helm_remote', 'helm_remote')
load('ext://helm_resource', 'helm_resource', 'helm_repo')

# Commenting out because this is not needed for now
# git_checkout('REDACTED', checkout_dir="tilt_git/common_tilt")
# _mydir = os.path.abspath(os.path.dirname(__file__))
# load('tilt_git/common_tilt/Tiltfile', 'setup_harbor_repo')
namespace_create('glrunner')
local_resource('Go Deps', cmd='make vendor', deps=['Makefile', 'go.mod', 'go.sum'])
local_resource('Build manifests', cmd='make manifests', deps=['Makefile', 'make/*'])
local_resource('Generate CRDs', cmd='make generate', deps=['Makefile', 'make/*'])
local_resource('Install CRDs', cmd='make install', deps=['Makefile', 'make/*'])

docker_build('localhost:5000/dolt-operator', '.', dockerfile="Dockerfile")
docker_build('localhost:5000/dolt-operator-test-runner', '.', dockerfile="Dockerfile.dev")
#k8s_yaml(kustomize('config/default'))

k8s_yaml(['hack/manifests/e2e/cluster-role.yaml', 'hack/manifests/storageclass.yaml'])
k8s_resource(
  objects=[
    'standard-resize:storageclass',
    'dolt-operator-test-runner-sa', 
    'dolt-operator-test-runner-clusterrole', 
    'dolt-operator-test-runner-clusterrolebinding'
  ],
  new_name='Test Runner Config'
)

k8s_yaml('hack/manifests/e2e/test-runner.yaml')
k8s_resource(
  workload='dolt-operator-test-runner-job',
  new_name='Test Runner Execution 1'
)
