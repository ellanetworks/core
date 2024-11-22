# Define variables
UI_DIR := ui
GO_CMD := cmd/ella/main.go
OUTPUT := ella
ROCK_FILE := ella_0.0.4_amd64.rock
TAR_FILE := ella.tar
K8S_IMAGE_PATH := /var/snap/k8s/common/images/
CONFIG_FILE := config/config.yaml
K8S_NAMESPACE := dev2
OCI_IMAGE_NAME := ella:0.0.4
N3_NAD := k8s/n3-nad.yaml
N6_NAD := k8s/n6-nad.yaml
MONGODB_DEPLOYMENT := k8s/mongodb-deployment.yaml
MONGODB_SERVICE := k8s/mongodb-service.yaml
MONGODB_CONFIGMAP := k8s/mongodb-configmap.yaml
GNBSIM_DEPLOYMENT := k8s/gnbsim-deployment.yaml
GNBSIM_CONFIGMAP := k8s/gnbsim-configmap.yaml
DEPLOYMENT := k8s/deployment.yaml

.PHONY: all clean build ui-build go-build oci-build deploy nad-create rebuild

# Default target
all: build

# Build everything
build: ui-build go-build oci-build
	@echo "Build completed successfully."

# Build the UI
ui-build:
	@echo "Installing and building UI..."
	npm install --prefix $(UI_DIR)
	npm run build --prefix $(UI_DIR)

# Build the Go application
go-build:
	@echo "Building Go application..."
	go build -o $(OUTPUT) $(GO_CMD)

# Build the OCI image
oci-build:
	@echo "Building OCI image..."
	rockcraft pack
	@echo "Copying OCI image to Docker daemon with skopeo..."
	sudo rockcraft.skopeo --insecure-policy copy oci-archive:$(ROCK_FILE) docker-daemon:$(OCI_IMAGE_NAME)
	@echo "Pushing image to local registry..."
	docker tag ella:0.0.4 localhost:5000/ella:0.0.4
	docker push localhost:5000/ella:0.0.4

# Deploy MongoDB
mongodb-deploy:
	@echo "Deploying MongoDB..."
	kubectl apply -f $(MONGODB_CONFIGMAP)
	kubectl apply -f $(MONGODB_SERVICE)
	kubectl apply -f $(MONGODB_DEPLOYMENT)

# Deploy gnbsim
gnbsim-deploy:
	@echo "Deploying gnbsim..."
	kubectl apply -f $(GNBSIM_CONFIGMAP)
	kubectl apply -f $(GNBSIM_DEPLOYMENT)

# Deploy the container to local Kubernetes
deploy: oci-build mongodb-deploy gnbsim-deploy
	@echo "Creating Network Attachment Definitions..."
	kubectl apply -f $(N3_NAD)
	kubectl apply -f $(N6_NAD)
	@echo "Deploying Ella..."
	kubectl create configmap ella-config --namespace=$(K8S_NAMESPACE) --from-file=$(CONFIG_FILE) --dry-run=client -o yaml | kubectl apply -f -
	kubectl apply -n $(K8S_NAMESPACE) -f $(DEPLOYMENT)
	@echo "Deployment completed successfully."

# Clean the build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf $(OUTPUT)
	rm -rf $(UI_DIR)/node_modules
	rm -rf $(UI_DIR)/dist
	rm -f $(ROCK_FILE) $(TAR_FILE)

# Helper target for cleaning and rebuilding
rebuild: clean build
	@echo "Rebuild completed."
