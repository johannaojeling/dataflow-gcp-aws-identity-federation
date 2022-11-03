# Dataflow GCP AWS Identity Federation

## Introduction

This repository contains a step-by-step guide and resources for setting
up [identity federation](https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_providers_oidc.html) between GCP and
AWS to allow Dataflow workers to access resources on AWS by using temporary security credentials. The examples are for
the Apache Beam Go SDK.

The document goes through the following steps:

1. Set up a GCP service account for Dataflow and an AWS role to assume
2. Build a custom Apache Beam image with support for identity federation
3. Run a Dataflow pipeline using the image

The [gcloud CLI](https://cloud.google.com/sdk/gcloud) and [AWS CLI](https://aws.amazon.com/cli/) will be used.

## 1. Set up a GCP service account for Dataflow and an AWS role to assume

This section outlines the process to set up identity federation between GCP and AWS, which consists of the following
steps:

1. Create a service account in GCP for Dataflow
2. Create a role in AWS for the Dataflow service account to assume

The following GCP variables are used:

| Variable | Description                                                                               |
|----------|-------------------------------------------------------------------------------------------|
| PROJECT  | Project                                                                                   |
| SA_NAME  | Name of Dataflow service account                                                          |
| SA_EMAIL | Email of Dataflow service account, set to `${SA_NAME}@${PROJECT}.iam.gserviceaccount.com` |

The following AWS variables are used:

| Variable   | Description                                                                                            |
|------------|--------------------------------------------------------------------------------------------------------|
| ROLE_NAME  | Name of role to be assumed by the Dataflow service account                                             |
| POLICY_ARN | ARN of permissions policy to attach to the role, e.g. `arn:aws:iam::aws:policy/AmazonS3ReadOnlyAccess` |

### Create a GCP service account

Create a service account to be used for Dataflow:

```bash
gcloud iam service-accounts create ${SA_NAME}
```

Assign roles to the service account, which must be at least:

- `roles/dataflow.worker`
- `roles/iam.serviceAccountTokenCreator`
- `roles/storage.objectAdmin`

```bash
ROLES=(
  "roles/dataflow.worker"
  "roles/iam.serviceAccountTokenCreator"
  "roles/storage.objectAdmin"
)
for role in "${ROLES[@]}"
do
  gcloud projects add-iam-policy-binding ${PROJECT} \
  --member="serviceAccount:${SA_EMAIL}" \
  --role="${role}"
done
```

Retrieve the service account's unique id:

```bash
UNIQUE_ID=$(gcloud iam service-accounts describe ${SA_EMAIL} --format="value(uniqueId)")
````

### Create an AWS role

Create a trust policy document with the web identity provider set to Google and the audience set to the Dataflow service
account's unique id retrieved in the previous step:

```bash
POLICY='{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Federated": "accounts.google.com"
      },
      "Action": "sts:AssumeRoleWithWebIdentity",
      "Condition": {
        "StringEquals": {
          "accounts.google.com:aud": "%s"
        }
      }
    }
  ]
}
'

POLICY_FILE="trust-policy.json"
printf ${POLICY} ${UNIQUE_ID} > ${POLICY_FILE}
```

Create a role with the trust policy. The maximum session duration in this example is set to one hour:

```bash
aws iam create-role \
--role-name=${ROLE_NAME} \
--max-session-duration=3600 \
--assume-role-policy-document="file://${POLICY_FILE}"
```

Attach a policy to the role with the resource permissions needed:

```bash
aws iam attach-role-policy \
--role-name=${ROLE_NAME} \
--policy-arn=${POLICY_ARN}
```

Retrieve the role's ARN:

```bash
AWS_ROLE_ARN=$(aws iam get-role --role-name=${ROLE_NAME} --query="Role.Arn" --output=text)
```

## 2. Build a custom Apache Beam image with support for identity federation

This section describes how to create a custom Apache Beam Go SDK image that allows Dataflow workers to assume an AWS
role. The image is configured to run a script on container startup that generates temporary AWS credentials before
running the default Apache Beam boot script.

The script [main.go](main.go) does the following:

1. Generates a Google-signed OIDC ID token for the Dataflow service account
2. Assumes an AWS role with the ID token and receives temporary credentials
3. Writes the credentials to `~/.aws/credentials`

The additional variables are used:

| Variable     | Description                                                    |
|--------------|----------------------------------------------------------------|
| IMAGE_URI    | URI of container image, e.g. `gcr.io/${PROJECT}/my-beam-image` |
| BUILD_BUCKET | Bucket used for Cloud Build                                    |
| AWS_REGION   | AWS region                                                     |

Build image with Cloud Build:

```bash
gcloud builds submit . \
--gcs-source-staging-dir=gs://${BUILD_BUCKET}/staging \
--substitutions="_IMAGE_URI=${IMAGE_URI},\
_LOGS_DIR=gs://${BUILD_BUCKET}/logs,\
_AWS_ROLE_ARN=${AWS_ROLE_ARN},\
_AWS_REGION=${AWS_REGION}"
```

## 3. Run a Dataflow pipeline using the image

To run a pipeline with DataflowRunner using the image built in the previous step, execute the following:

```bash
go run my_pipeline.go \
--runner=dataflow \
--service_account_email=${SA_EMAIL} \
--sdk_container_image=${IMAGE_URI} \
[...] # other pipeline options
```
