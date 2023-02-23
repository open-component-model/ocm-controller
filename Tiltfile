# -*- mode: Python -*-

kubectl_cmd = "kubectl"
flux_cmd = "flux"

# verify kubectl command exists
if str(local("command -v " + kubectl_cmd + " || true", quiet = True)) == "":
    fail("Required command '" + kubectl_cmd + "' not found in PATH")

# set defaults
settings = {
    "flux": {
        "enabled": False,
        "bootstrap": True,
        "repository": os.getenv("FLUX_REPOSITORY", "podinfo-flux-example"),
        "owner": os.getenv("FLUX_OWNER", ""),
        "path": os.getenv("FLUX_PATH", "."),
    },
    "install_unpacker": {
        "enabled": False,
        "path": "",
    },
    "create_secrets": {
        "enable": True,
        "token": os.getenv("GITHUB_TOKEN", ""),
        "email": os.getenv("GITHUB_EMAIL", ""),
        "user": os.getenv("GITHUB_USER", ""),
    },
}

# global settings
tilt_file = "./tilt-settings.yaml" if os.path.exists("./tilt-settings.yaml") else "./tilt-settings.json"
settings.update(read_yaml(
    tilt_file,
    default = {},
))
load('ext://secret', 'secret_yaml_registry', 'secret_from_dict')


def bootstrap_or_install_flux():
    opts = settings.get("flux")
    if not opts.get("enabled"):
        return

    if str(local("command -v " + flux_cmd + " || true", quiet = True)) == "":
        fail("Required command '" + flux_cmd + "' not found in PATH")

    # flux bootstrap github --owner=${FLUX_OWNER} --repository=${FLUX_REPOSITORY} --path ${FLUX_PATH}
    if opts.get("bootstrap"):
        local("%s bootstrap github --owner %s --repository %s --path %s" % (flux_cmd, opts.get('owner'), opts.get('repository'), opts.get('path')))
    else:
        local(flux_cmd + " install")


def install_unpacker():
    opts = settings.get("install_unpacker")
    if not opts.get("enabled"):
        return


def create_secrets():
    opts = settings.get("create_secrets")
    if not opts.get("enable"):
        return

    k8s_yaml(secret_yaml_registry("regcred", "ocm-system", flags_dict = {
        'docker-server': 'ghcr.io',
        'docker-username': opts.get('user'),
        'docker-email': opts.get('email'),
        'docker-password': opts.get('token'),
    }))
    k8s_yaml(secret_from_dict("creds", "ocm-system", inputs = {
        'username' : opts.get('user'),
        'password' : opts.get('token'),
    }))


# set up the development environment

# check if flux is needed
bootstrap_or_install_flux()

# check if installing unpacker is needed
install_unpacker()

# Use kustomize to build the install yaml files
install = kustomize('config/default')

# Update the root security group. Tilt requires root access to update the
# running process.
objects = decode_yaml_stream(install)
for o in objects:
    if o.get('kind') == 'Deployment' and o.get('metadata').get('name') == 'ocm-controller':
        o['spec']['template']['spec']['securityContext']['runAsNonRoot'] = False
        break

updated_install = encode_yaml_stream(objects)

# Apply the updated yaml to the cluster.
k8s_yaml(updated_install)

# Create Secrets
create_secrets()

load('ext://restart_process', 'docker_build_with_restart')

# enable hot reloading by doing the following:
# - locally build the whole project
# - create a docker imagine using tilt's hot-swap wrapper
# - push that container to the local tilt registry
# Once done, rebuilding now should be a lot faster since only the relevant
# binary is rebuilt and the hot swat wrapper takes care of the rest.
local_resource(
    'manager',
    'CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/manager ./',
    deps = [
        "main.go",
        "go.mod",
        "go.sum",
        "api",
        "controllers",
        "pkg",
    ],
)

# Build the docker image for our controller. We use a specific Dockerfile
# since tilt can't run on a scratch container.
docker_build_with_restart(
    'ghcr.io/open-component-model/ocm-controller',
    '.',
    dockerfile = 'tilt.dockerfile',
    entrypoint = '/bin/manager',
    only=[
      './bin',
    ],
    live_update = [
        sync('./bin', '/bin'),
    ],
)

