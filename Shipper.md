Idea is to deploy a light weight jump server which would make api call in the nomad VPC with url on 10.10.85.1 and make a deployment of a service. 

it will expose two endpoint
1. health
2  Deploy and the service name with secret key of 64 characters
3. it will validate the secret key and based on that call the corresponding nomad services to launch the deployment with a tag id in the meta. this will be automated ci/cd