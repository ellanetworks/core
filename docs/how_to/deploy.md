# Deploy

Ella Core is available as a Snap, a OCI container image, and a Juju Kubernetes charm.

=== "Snap (Recommended)"

    Ella Core is available as a snap package [here](https://snapcraft.io/ella-core).

    Install the snap:

    ```bash
    sudo snap install ella-core --channel=edge --devmode
    ```

    Generate (or copy) a certificate and private key to the following location:

    ```bash
    sudo openssl req -newkey rsa:2048 -nodes -keyout /var/snap/ella-core/common/key.pem -x509 -days 1 -out /var/snap/ella-core/common/cert.pem -subj "/CN=example.com"
    ```

    Start the service:
    ```bash
    sudo snap start ella-core.cored
    ```

    Navigate to `https://localhost:5000` to access the Ella UI.


=== "Kubernetes and Juju"
        
    Install MicroK8s:

    ```shell
    sudo snap install microk8s --channel=1.31/stable --classic
    ```

    Add the necessary MicroK8s addons:

    ```shell
    sudo microk8s addons repo add community https://github.com/canonical/microk8s-community-addons --reference feat/strict-fix-multus
    sudo microk8s enable hostpath-storage
    sudo microk8s enable multus
    ```

    Install Juju:

    ```shell
    sudo snap install juju
    ```

    Bootstrap a Juju controller:

    ```shell
    juju bootstrap microk8s
    ```

    Create a Juju model:

    ```shell
    juju add-model ella-core
    ```

    ```shell
    juju deploy ella-core-k8s --trust
    ```
