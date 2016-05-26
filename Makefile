APP := artifact-capture-kubernetes
REGISTRY := $(REGISTRY)

ifeq ($(CIRCLE_BUILD_NUM),)
	BUILD := dev
	KUBE_ENV := development
else
	BUILD := $(CIRCLE_BUILD_NUM)
	KUBE_ENV := production
endif

DOCKER_IMAGE := $(REGISTRY)/$(APP):$(BUILD)

all: build push deploy

build:
	go get ./...
	GOOS=linux CGO_ENABLED=0 go build --ldflags '-extldflags "-static"' -o ack src/main.go
	docker build -t $(DOCKER_IMAGE) .

push:
	docker push $(DOCKER_IMAGE)

deploy: push update_deployment

update_deployment:
	-kubectl delete rc $(APP) --namespace=$(KUBE_ENV)
	sleep 60

	sed -e "s#__BUILD__#$(BUILD)#" \
		-e "s#__CONTAINER_NAME__#$(APP)#" \
		-e "s#__APP__#$(APP)#" \
		-e "s#__IMAGE__#$(DOCKER_IMAGE)#" \
		-e "s#__GRID_IP__#selenium-hub#" \
		-e "s#__GCLOUD_PROJECT_SECRET__#$(GCLOUD_PROJECT_SECRET)#" \
		-e "s#__GCLOUD_PROJECT_ID__#$(GCLOUD_PROJECT_ID)#" \
		-e "s#__GCLOUD_BUCKET__#$(GCLOUD_BUCKET)#" \
		-e "s#__GCLOUD_KEY__#$(GCLOUD_KEY)#" \
		-e "s#__IC_API_KEY__#$(IC_API_KEY)#" \
	scripts/k8s/ack-rc.yml \
	| kubectl apply --namespace=$(KUBE_ENV) -f -

deps-circle:
	bash scripts/gcloud_setup.sh
	bash scripts/install-go.sh

fix_circle_go:
	scripts/install-go.sh

deploy-selenium:
	kubectl apply --filename=scripts/k8s/selenium-hub-rc.yml --namespace=$(KUBE_ENV)
	kubectl apply --filename=scripts/k8s/selenium-hub-service.yml --namespace=$(KUBE_ENV)
	kubectl apply --filename=scripts/k8s/selenium-agent-rc-firefox.yml --namespace=$(KUBE_ENV)

delete-selenium:
	kubectl delete --filename=scripts/k8s/selenium-hub-rc.yml --namespace=$(KUBE_ENV)
	kubectl delete --filename=scripts/k8s/selenium-hub-service.yml --namespace=$(KUBE_ENV)
	kubectl delete --filename=scripts/k8s/selenium-agent-rc-firefox.yml --namespace=$(KUBE_ENV)

selenium-port-forward:
	$(eval PODNAME:=$(shell kubectl get pods --selector="app=selenium-hub" --output=template --template="{{with index .items 0}}{{.metadata.name}}{{end}}" --namespace=$(KUBE_ENV)))
	kubectl port-forward $(PODNAME) 4444:4444 --namespace=$(KUBE_ENV)

help: ## print list of tasks and descriptions
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

.DEFAULT_GOAL := help
