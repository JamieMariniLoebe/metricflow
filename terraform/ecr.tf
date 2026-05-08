module "ecr" {
  source  = "terraform-aws-modules/ecr/aws"
  version = "~> 3.0"

  repository_name = "metricflow"

  repository_image_tag_mutability = "IMMUTABLE"
  repository_force_delete         = true
  repository_lifecycle_policy = jsonencode({
    rules = [
      {
        rulePriority = 1,
        description  = "Keep last 30 images",
        selection = {
          tagStatus   = "any",
          countType   = "imageCountMoreThan",
          countNumber = 30
        }
        action = { type = "expire" }
      }
    ]
  })
  tags = {
    Terraform   = "true"
    Project     = "metricflow"
    Environment = "dev"
  }
}
