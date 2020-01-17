GO111MODULE ?= on
export GO111MODULE

helpers:
	docker build ../../images/resource-engine-helm2 --tag 'crossplane/resource-engine-helm2:master'
	docker build ../../images/resource-engine-kubectl --tag 'crossplane/resource-engine-kubectl:master'
.PHONY: helpers

integration-test: helpers integration-test-helm2
.PHONY: integration-test

integration-test-helm2:
	docker build helm2 --tag 'crossplane/sample-stack-claim-test:helm2'
	kubectl apply -f helm2/sample-clusterinstall.yaml
	@echo "Giving the stack manager time to install our stack . . ."
	sleep 10
	@echo "If the CRD doesn't exist yet, try running the test again."
	kubectl apply -f helm2/sample-cr.yaml
	@echo "Printing test object statuses"
	kubectl get job -A
	kubectl get pod -A
	@echo "Giving the controller some time to process our resources . . ."
	sleep 10
	@echo "If the config map 'mycustomname-helm2' isn't found, try looking for it again, or inspect the job logs to debug."
	kubectl get configmap mycustomname-helm2 -o yaml

.PHONY: integration-test-helm2

clean-integration-test: clean-integration-test-helm2

.PHONY: clean-integration-test

clean-integration-test-helm2:
	-kubectl delete configmap mycustomname-helm2
	-kubectl delete -f helm2/sample-cr.yaml
	-kubectl delete -f helm2/sample-clusterinstall.yaml

.PHONY: clean-integration-test-helm2
