UI_DIR := ui
GO_CMD := cmd/ella/main.go
OUTPUT := ella
ROCK_FILE := ella_0.0.4_amd64.rock
TAR_FILE := ella.tar
K8S_NAMESPACE := dev2
OCI_IMAGE_NAME := ella:0.0.4
MONGODB_DEPLOYMENT := k8s/mongodb-deployment.yaml
MONGODB_SERVICE := k8s/mongodb-service.yaml
MONGODB_CONFIGMAP := k8s/mongodb-configmap.yaml
GNBSIM_GNB_NAD := k8s/gnbsim-gnb-nad.yaml
GNBSIM_DEPLOYMENT := k8s/gnbsim-deployment.yaml
GNBSIM_SERVICE := k8s/gnbsim-service.yaml
GNBSIM_CONFIGMAP := k8s/gnbsim-configmap.yaml
ROUTER_RAN_NAD := k8s/router-ran-nad.yaml
ROUTER_CORE_NAD := k8s/router-core-nad.yaml
ROUTER_ACCESS_NAD := k8s/router-access-nad.yaml
ROUTER_DEPLOYMENT := k8s/router-deployment.yaml
ELLA_N3_NAD := k8s/ella-n3-nad.yaml
ELLA_N6_NAD := k8s/ella-n6-nad.yaml
ELLA_DEPLOYMENT := k8s/ella-deployment.yaml
ELLA_CONFIGMAP := k8s/ella-configmap.yaml
ELLA_SERVICE := k8s/ella-service.yaml

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
	docker tag ella:0.0.4 localhost:5000/ella:0.0.4
	docker push localhost:5000/ella:0.0.4

mongodb-deploy:
	@echo "Deploying MongoDB..."
	kubectl apply -f $(MONGODB_CONFIGMAP)
	kubectl apply -f $(MONGODB_SERVICE)
	kubectl apply -f $(MONGODB_DEPLOYMENT)

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

ella-deploy: wait-for-mongodb
	@echo "Deploying Ella..."
	kubectl apply -f $(ELLA_N3_NAD)
	kubectl apply -f $(ELLA_N6_NAD)
	kubectl apply -f $(ELLA_CONFIGMAP)
	kubectl apply -f $(ELLA_DEPLOYMENT)
	kubectl apply -f $(ELLA_SERVICE)
	kubectl exec -i $$(kubectl get pods -n $(K8S_NAMESPACE) -l app=ella -o jsonpath="{.items[0].metadata.name}") -n $(K8S_NAMESPACE) -- pebble start ella
	@echo "Ella deployment completed successfully."

wait-for-mongodb:
	@echo "Waiting for MongoDB to be ready..."
	while ! kubectl wait --namespace $(K8S_NAMESPACE) --for=condition=ready pod -l app=mongodb --timeout=30s; do \
		echo "MongoDB is not ready yet. Retrying..."; \
		sleep 2; \
	done
	@echo "MongoDB is ready."

deploy: mongodb-deploy gnbsim-deploy router-deploy ella-deploy
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
