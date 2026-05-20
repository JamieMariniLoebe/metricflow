resource "aws_security_group" "rds" {
  name        = "metricflow-rds"
  description = "Allow Postgres ingress from EKS nodes"
  vpc_id      = module.vpc.vpc_id

  tags = {
    Terraform   = "true"
    Project     = "metricflow"
    Environment = "dev"
  }
}

resource "aws_security_group_rule" "rds_ingress_from_eks" {
  type                     = "ingress"
  from_port                = 5432
  to_port                  = 5432
  protocol                 = "tcp"
  security_group_id        = aws_security_group.rds.id
  source_security_group_id = module.eks.node_security_group_id
}

resource "aws_db_subnet_group" "metricflow" {
  name       = "metricflow-rds-subnets"
  subnet_ids = module.vpc.private_subnets

  tags = {
    Terraform   = "true"
    Project     = "metricflow"
    Environment = "dev"
  }
}

resource "aws_db_parameter_group" "metricflow" {
  name   = "metricflow-rds-parameter-group"
  family = "postgres16"

  tags = {
    Terraform   = "true"
    Project     = "metricflow"
    Environment = "dev"
  }
}

resource "aws_db_instance" "metricflow" {
  allocated_storage           = 20
  db_name                     = "metricflow"
  engine                      = "postgres"
  engine_version              = "16.14"
  instance_class              = "db.t4g.micro"
  manage_master_user_password = true
  username                    = "metricflow"
  parameter_group_name        = aws_db_parameter_group.metricflow.name
  db_subnet_group_name        = aws_db_subnet_group.metricflow.name
  vpc_security_group_ids      = [aws_security_group.rds.id]
  skip_final_snapshot         = true
  storage_type                = "gp3"
  storage_encrypted           = true
  backup_retention_period     = 7
  backup_window               = "08:00-09:00"
  deletion_protection         = false
  identifier                  = "metricflow"
  publicly_accessible         = false


  tags = {
    Terraform   = "true"
    Project     = "metricflow"
    Environment = "dev"
  }
}
