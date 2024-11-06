load('ext://git_resource', 'git_checkout')
load('ext://namespace', 'namespace_create', 'namespace_inject')
load('ext://helm_remote', 'helm_remote')
load('ext://helm_resource', 'helm_resource', 'helm_repo')
git_checkout('REDACTED', checkout_dir="tilt_git/common_tilt")
_mydir = os.path.abspath(os.path.dirname(__file__))

load('tilt_git/common_tilt/Tiltfile', 'install_istio', 'setup_harbor_repo')

local_resource('Build manifests', cmd='make manifests', deps=['Makefile', 'make/*'])
local_resource('Generate CRDs', cmd='make generate', deps=['Makefile', 'make/*'])
local_resource('Install CRDs', cmd='make install', deps=['Makefile', 'make/*'])

docker_build('localhost:5000/dolt-operator-tests', '.', dockerfile="Dockerfile.dev")
k8s_yaml(['hack/manifests/e2e/cluster-role.yaml', 'hack/manifests/e2e/test-runner.yaml'])
