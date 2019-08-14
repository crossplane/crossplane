#################################
## Variables
resource "random_id" "this" {
  byte_length = 2
  prefix      = "crossplane-example-"
}

variable "aws_cred_file" {
  default = "~/.aws/credentials"
}

variable "aws_region" {
  default = "us-west-2"
}

variable "subnet_cidr_blocks" {
  description = "A list of subnet cidr blocks inside the VPC"
  type        = "list"
  default     = ["192.168.64.0/18", "192.168.128.0/18", "192.168.192.0/18"]
}
#################################
## Providers
provider "aws" {
  region = "${var.aws_region}"
}

provider "local" {
}

provider "template" {
}

#################################
## Networking
data "aws_availability_zones" "available" {
  state = "available"
}

### VPC
resource "aws_vpc" "this" {
  cidr_block           = "192.168.0.0/16"
  enable_dns_support   = true
  enable_dns_hostnames = true
  tags = {
    Name = "VPC_${random_id.this.hex}"
  }
}

resource "aws_security_group" "eks" {
  name        = "eks_${random_id.this.hex}"
  description = "Cluster communication with worker nodes"
  vpc_id      = "${aws_vpc.this.id}"
}

### Subnets
resource "aws_subnet" "this" {

  count             = 3
  vpc_id            = "${aws_vpc.this.id}"
  availability_zone = "${data.aws_availability_zones.available.names[count.index]}"
  cidr_block        = "${element(var.subnet_cidr_blocks, count.index)}"
  tags = {
    Name = "Subnet_${count.index}_${random_id.this.hex}"
  }
}

### Internet Gateway
resource "aws_internet_gateway" "this" {
  vpc_id = "${aws_vpc.this.id}"

  tags = {
    Name = "IG_${random_id.this.hex}"
  }
}

resource "aws_route_table" "this" {
  vpc_id = "${aws_vpc.this.id}"

  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = "${aws_internet_gateway.this.id}"
  }
}

resource "aws_route_table_association" "this" {
  count = 3

  subnet_id      = "${aws_subnet.this.*.id[count.index]}"
  route_table_id = "${aws_route_table.this.id}"
}

#################################
## EC2
### Key Pair
resource "tls_private_key" "this" {
  algorithm = "RSA"
  rsa_bits  = 2048
}

resource "aws_key_pair" "this" {
  key_name   = "${random_id.this.hex}"
  public_key = "${tls_private_key.this.public_key_openssh}"
}

#################################
## IAM
resource "aws_iam_role" "this" {
  name = "${random_id.this.hex}"

  assume_role_policy = <<POLICY
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Service": "eks.amazonaws.com"
      },
      "Action": "sts:AssumeRole"
    }
  ]
}
POLICY
}

resource "aws_iam_role_policy_attachment" "EKSClusterPolicy" {
  policy_arn = "arn:aws:iam::aws:policy/AmazonEKSClusterPolicy"
  role       = "${aws_iam_role.this.name}"
}

resource "aws_iam_role_policy_attachment" "EKSServicePolicy" {
  policy_arn = "arn:aws:iam::aws:policy/AmazonEKSServicePolicy"
  role       = "${aws_iam_role.this.name}"
}

#################################
## RDS
resource "aws_db_subnet_group" "this" {
  name       = "${random_id.this.hex}"
  subnet_ids = "${aws_subnet.this.*.id}"

  tags = {
    Name = "rds_subnet_group_${random_id.this.hex}"
  }
}

resource "aws_security_group" "rds" {
  name        = "rds_${random_id.this.hex}"
  description = "Allow TLS inbound traffic"
  vpc_id      = "${aws_vpc.this.id}"
  ingress {
    from_port   = 3306
    to_port     = 3306
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

#################################
## populate the template
data "template_file" "this" {
  template = "${file("provider.yaml")}"

  vars = {
    BASE64ENCODED_AWS_PROVIDER_CREDS = "${replace(base64encode(file(var.aws_cred_file)), "\n", "")}"
    EKS_WORKER_KEY_NAME              = "${aws_key_pair.this.key_name}"
    EKS_SECURITY_GROUP_ID            = "${aws_security_group.eks.id}"
    EKS_VPC                          = "${aws_vpc.this.id}"
    REGION                           = "${var.aws_region}"
    RDS_SECURITY_GROUP_ID            = "${aws_security_group.rds.id}"
    RDS_SUBNET_GROUP_NAME            = "${aws_db_subnet_group.this.name}"
    EKS_SUBNETS                      = "${join(",", aws_subnet.this.*.id)}"
    EKS_ROLE_ARN                     = "${aws_iam_role.this.arn}"
  }

  depends_on = [
    "aws_vpc.this"
  ]
}

output "provider" {
  value     = "${data.template_file.this.rendered}"
  sensitive = true
}
