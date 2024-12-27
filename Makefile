UI_DIR := ui
GO_CMD := cmd/core/main.go
OUTPUT := core
ROCK_FILE := ella-core_0.0.4_amd64.rock
TAR_FILE := ella.tar
K8S_NAMESPACE := dev2
OCI_IMAGE_NAME := ella-core:0.0.4
GNBSIM_GNB_NAD := k8s/gnbsim-gnb-nad.yaml
GNBSIM_DEPLOYMENT := k8s/gnbsim-deployment.yaml
GNBSIM_SERVICE := k8s/gnbsim-service.yaml
GNBSIM_CONFIGMAP := k8s/gnbsim-configmap.yaml
ROUTER_RAN_NAD := k8s/router-ran-nad.yaml
ROUTER_CORE_NAD := k8s/router-core-nad.yaml
ROUTER_ACCESS_NAD := k8s/router-access-nad.yaml
ROUTER_DEPLOYMENT := k8s/router-deployment.yaml
CORE_N3_NAD := k8s/core-n3-nad.yaml
CORE_N6_NAD := k8s/core-n6-nad.yaml
CORE_DEPLOYMENT := k8s/core-deployment.yaml
CORE_CONFIGMAP := k8s/core-configmap.yaml
CORE_SERVICE := k8s/core-service.yaml

.PHONY: all clean build ui-build go-build oci-build deploy nad-create rebuild

all: build

build: ui-build go-build oci-build
	@echo "Build completed successfully."

ui-build:
	@echo "Installing and building UI..."
	npm install --prefix $(UI_DIR)
	npm run build --prefix $(UI_DIR)

go-build:
	@echo "Building Go application..."
	go build -o $(OUTPUT) $(GO_CMD)

oci-build:
	@echo "Building OCI image..."
	rockcraft pack
	@echo "Copying OCI image to Docker daemon with skopeo..."
	sudo rockcraft.skopeo --insecure-policy copy oci-archive:$(ROCK_FILE) docker-daemon:$(OCI_IMAGE_NAME)
	@echo "Pushing image to local registry..."
	docker tag ella-core:0.0.4 localhost:5000/ella-core:0.0.4
	docker push localhost:5000/ella-core:0.0.4

gnbsim-deploy:
	@echo "Deploying gnbsim..."
	kubectl apply -f $(GNBSIM_GNB_NAD)
	kubectl apply -f $(GNBSIM_CONFIGMAP)
	kubectl apply -f $(GNBSIM_DEPLOYMENT)
	kubectl apply -f $(GNBSIM_SERVICE)

router-deploy:
	@echo "Deploying router..."
	kubectl apply -f $(ROUTER_RAN_NAD)
	kubectl apply -f $(ROUTER_CORE_NAD)
	kubectl apply -f $(ROUTER_ACCESS_NAD)
	kubectl apply -f $(ROUTER_DEPLOYMENT)

core-deploy: 
	@echo "Deploying Ella..."
	kubectl apply -f $(CORE_N3_NAD)
	kubectl apply -f $(CORE_N6_NAD)
	kubectl apply -f $(CORE_CONFIGMAP)
	kubectl apply -f $(CORE_DEPLOYMENT)
	kubectl apply -f $(CORE_SERVICE)
	@echo "Ella deployment completed successfully."

wait-for-ella-core:
	@echo "Waiting for Ella to be ready..."
	while ! kubectl wait --namespace $(K8S_NAMESPACE) --for=condition=ready pod -l app=ella-core --timeout=30s; do \
		echo "Ella is not ready yet. Retrying..."; \
		sleep 2; \
	done
	@echo "Ella is ready."

core-start: wait-for-ella-core
	@echo "Starting Ella Core..."
	@POD_NAME=$$(kubectl get pods -n $(K8S_NAMESPACE) -l app=ella-core -o jsonpath="{.items[0].metadata.name}"); \
    kubectl exec -i $$POD_NAME -n $(K8S_NAMESPACE) -- pebble add ella-core /config/pebble.yaml; \
	kubectl exec -i $$POD_NAME -n $(K8S_NAMESPACE) -- pebble start ella-core

deploy: gnbsim-deploy router-deploy core-deploy core-start
	@echo "Deployment completed successfully."

hotswap: go-build
	@echo "Copying the binary to the running container..."
	@POD_NAME=$$(kubectl get pods -n $(K8S_NAMESPACE) -l app=ella -o jsonpath="{.items[0].metadata.name}"); \
	CONTAINER_NAME=$$(kubectl get pod $$POD_NAME -n $(K8S_NAMESPACE) -o jsonpath="{.spec.containers[0].name}"); \
	kubectl cp $(OUTPUT) $$POD_NAME:/bin/ella -c $$CONTAINER_NAME -n $(K8S_NAMESPACE); \
	kubectl exec -i $$POD_NAME -n $(K8S_NAMESPACE) -c $$CONTAINER_NAME -- pebble restart ella
	@echo "Hotswap completed successfully."

test:
	@echo "Running end-to-end tests..."
	tox -e integration
	@echo "End-to-end tests completed successfully."

hotswap-test: hotswap test

clean:
	@echo "Cleaning build artifacts..."
	rm -rf $(OUTPUT)
	rm -rf $(UI_DIR)/node_modules
	rm -rf $(UI_DIR)/dist
	rm -f $(ROCK_FILE) $(TAR_FILE)

rebuild: clean build
	@echo "Rebuild completed."
