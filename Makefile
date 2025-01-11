UI_DIR := ui
GO_CMD := cmd/core/main.go
OUTPUT := core
ROCK_FILE := ella-core_0.0.4_amd64.rock
TAR_FILE := ella.tar
K8S_NAMESPACE := dev2
OCI_IMAGE_NAME := ella-core:latest

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
	docker tag ella-core:latest localhost:5000/ella-core:latest
	docker push localhost:5000/ella-core:latest

hotswap: go-build
	@echo "Copying the binary to the running container..."
	@POD_NAME=$$(kubectl get pods -n $(K8S_NAMESPACE) -l app=ella-core -o jsonpath="{.items[0].metadata.name}"); \
	CONTAINER_NAME=$$(kubectl get pod $$POD_NAME -n $(K8S_NAMESPACE) -o jsonpath="{.spec.containers[0].name}"); \
	kubectl cp $(OUTPUT) $$POD_NAME:/bin/core -c $$CONTAINER_NAME -n $(K8S_NAMESPACE); \
	kubectl exec -i $$POD_NAME -n $(K8S_NAMESPACE) -c $$CONTAINER_NAME -- pebble restart ella-core
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
