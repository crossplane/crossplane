#!/bin/bash

# NOTE: You need to install yq for this script to work. "brew install yq" should do the job for macOS users.

echo "Running create-resource for RDS::DBCluster"
aws cloudcontrol create-resource --type-name "AWS::RDS::DBCluster" --desired-state "$(yq eval dbcluster.yaml -o=json)"
# An error occurred (UnsupportedActionException) when calling the CreateResource operation: Resource type AWS::RDS::DBCluster does not support CREATE action
echo "Completed create-resource for DBCluster"

echo "Running create-resource for ECR::Repository"
aws cloudcontrol create-resource --type-name "AWS::ECR::Repository" --desired-state "$(yq eval repository.yaml -o=json)"
# {
#     "ProgressEvent": {
#         "TypeName": "AWS::ECR::Repository",
#         "RequestToken": "08ac298e-7cea-4d5f-89f4-ee88a4674425",
#         "Operation": "CREATE",
#         "OperationStatus": "IN_PROGRESS",
#         "EventTime": "2021-10-12T17:58:01.583000+03:00"
#     }
# }
echo "Completed create-resource for ECR::Repository"

echo "Running get-resource-request-status for ECR::Repository"
echo "Make sure to update the request token."
aws cloudcontrol get-resource-request-status --request-token baece8d2-2f0e-46f7-8238-36dc59180f81
# {
#     "ProgressEvent": {
#         "TypeName": "AWS::ECR::Repository",
#         "Identifier": "muvaf-testing",
#         "RequestToken": "baece8d2-2f0e-46f7-8238-36dc59180f81",
#         "Operation": "CREATE",
#         "OperationStatus": "SUCCESS",
#         "EventTime": "2021-10-12T18:02:55.632000+03:00"
#     }
# }
echo "Completed get-resource-request-status for ECR::Repository"