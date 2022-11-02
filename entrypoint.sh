#!/bin/bash

# Generate AWS credentials
/pipeline/gen-aws-creds --roleArn="${AWS_ROLE_ARN}" --outputPath="${HOME}/.aws/credentials"

# Run Apache Beam boot script
/opt/apache/beam/boot "$@"
