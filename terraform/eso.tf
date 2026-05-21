data "aws_iam_policy_document" "eso_trust" {
  statement {
    effect  = "Allow"
    actions = ["sts:AssumeRoleWithWebIdentity"]

    principals {
      type        = "Federated"
      identifiers = [module.eks.oidc_provider_arn]
    }

    condition {
      test     = "StringEquals"
      variable = "${module.eks.oidc_provider}:aud"
      values   = ["sts.amazonaws.com"]
    }

    condition {
      test     = "StringEquals"
      variable = "${module.eks.oidc_provider}:sub"
      values   = ["system:serviceaccount:default:metricflow-eso"]
    }

  }
}

resource "aws_iam_role" "eso" {
  name               = "eso-metricflow"
  assume_role_policy = data.aws_iam_policy_document.eso_trust.json

  tags = {
    Terraform   = "true"
    Project     = "metricflow"
    Environment = "dev"
  }

}

data "aws_iam_policy_document" "eso_permissions" {
  statement {
    effect    = "Allow"
    actions   = ["secretsmanager:GetSecretValue", "secretsmanager:DescribeSecret"]
    resources = [aws_db_instance.metricflow.master_user_secret[0].secret_arn]
  }
}

resource "aws_iam_role_policy" "eso" {
  name   = "eso-secrets-access"
  role   = aws_iam_role.eso.id
  policy = data.aws_iam_policy_document.eso_permissions.json

}
