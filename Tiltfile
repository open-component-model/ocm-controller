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

# Deploy: tell tilt what files to deploy from
k8s_yaml(kustomize('config/default'))

# Create Secrets
create_secrets()

# build the main controller and inject it into the tilt registry
# bug with build with restart binary: https://github.com/tilt-dev/tilt/issues/6047
# load('ext://restart_process', 'docker_build_with_restart')
# docker_build_with_restart(
#     'ghcr.io/open-component-model/ocm-controller',
#     '.',
#     entrypoint = '/workspace/manager',
#     live_update = [
#         sync('./bin', '/workspace'),
#     ],
# )

docker_build(
    'ghcr.io/open-component-model/ocm-controller',
    context = '.',
    dockerfile = 'Dockerfile',
    live_update = [
        sync('./bin', '/workspace'),
    ],
)
