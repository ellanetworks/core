UI_DIR := ui
GO_CMD := cmd/core/main.go
OUTPUT := core
K8S_NAMESPACE := dev2

ui-build:
	@echo "Installing and building UI..."
	npm install --prefix $(UI_DIR)
	npm run build --prefix $(UI_DIR)

go-build:
	@echo "Building Go application..."
	go build -o $(OUTPUT) $(GO_CMD)

hotswap: go-build
	@echo "Copying the binary to the running container..."
	@POD_NAME=$$(kubectl get pods -n $(K8S_NAMESPACE) -l app=ella-core -o jsonpath="{.items[0].metadata.name}"); \
	CONTAINER_NAME=$$(kubectl get pod $$POD_NAME -n $(K8S_NAMESPACE) -o jsonpath="{.spec.containers[0].name}"); \
	kubectl cp $(OUTPUT) $$POD_NAME:/bin/core -c $$CONTAINER_NAME -n $(K8S_NAMESPACE); \
	kubectl exec -i $$POD_NAME -n $(K8S_NAMESPACE) -c $$CONTAINER_NAME -- pebble restart ella-core
	@echo "Hotswap completed successfully."
