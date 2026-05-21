output "rds_endpoint" {
  value = aws_db_instance.metricflow.endpoint
}

output "rds_master_user_secret_arn" {
  value     = aws_db_instance.metricflow.master_user_secret[0].secret_arn
  sensitive = true
}

output "eso_role_arn" {
  description = "IAM role ARN for the ESO ServiceAccount (IRSA)"
  value       = aws_iam_role.eso.arn
}
