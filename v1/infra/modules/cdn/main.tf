locals {
  use_custom_certificate = trimspace(var.certificate_arn) != ""
}

resource "aws_s3_bucket" "tower" {
  bucket        = "${var.environment}-nexus-tower"
  force_destroy = var.force_destroy_bucket

  tags = {
    Name = "${var.environment}-nexus-tower"
  }

  lifecycle {
    prevent_destroy = true
  }
}

resource "aws_s3_bucket_versioning" "tower" {
  bucket = aws_s3_bucket.tower.id

  versioning_configuration {
    status = "Enabled"
  }
}

resource "aws_s3_bucket_server_side_encryption_configuration" "tower" {
  bucket = aws_s3_bucket.tower.id

  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

resource "aws_s3_bucket_public_access_block" "tower" {
  bucket = aws_s3_bucket.tower.id

  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

resource "aws_cloudfront_origin_access_identity" "tower" {
  comment = "${var.environment}-nexus-tower-oai"
}

data "aws_iam_policy_document" "tower_oai" {
  statement {
    sid    = "AllowCloudFrontRead"
    effect = "Allow"

    principals {
      type        = "AWS"
      identifiers = [aws_cloudfront_origin_access_identity.tower.iam_arn]
    }

    actions   = ["s3:GetObject"]
    resources = ["${aws_s3_bucket.tower.arn}/*"]
  }
}

resource "aws_s3_bucket_policy" "tower" {
  bucket = aws_s3_bucket.tower.id
  policy = data.aws_iam_policy_document.tower_oai.json
}

resource "aws_cloudfront_distribution" "tower" {
  enabled             = true
  default_root_object = "index.html"
  is_ipv6_enabled     = true
  price_class         = "PriceClass_100"
  aliases             = trimspace(var.app_domain) == "" ? [] : [var.app_domain]

  origin {
    domain_name = aws_s3_bucket.tower.bucket_regional_domain_name
    origin_id   = "tower-s3"

    s3_origin_config {
      origin_access_identity = aws_cloudfront_origin_access_identity.tower.cloudfront_access_identity_path
    }
  }

  default_cache_behavior {
    allowed_methods  = ["GET", "HEAD"]
    cached_methods   = ["GET", "HEAD"]
    target_origin_id = "tower-s3"

    forwarded_values {
      query_string = false

      cookies {
        forward = "none"
      }
    }

    viewer_protocol_policy = "redirect-to-https"
    min_ttl                = 0
    default_ttl            = 300
    max_ttl                = 86400
    compress               = true
  }

  custom_error_response {
    error_code            = 404
    response_code         = 200
    response_page_path    = "/index.html"
    error_caching_min_ttl = 60
  }

  custom_error_response {
    error_code            = 403
    response_code         = 200
    response_page_path    = "/index.html"
    error_caching_min_ttl = 60
  }

  restrictions {
    geo_restriction {
      restriction_type = "none"
    }
  }

  viewer_certificate {
    acm_certificate_arn            = local.use_custom_certificate ? var.certificate_arn : null
    ssl_support_method             = local.use_custom_certificate ? "sni-only" : null
    minimum_protocol_version       = local.use_custom_certificate ? "TLSv1.2_2021" : null
    cloudfront_default_certificate = local.use_custom_certificate ? false : true
  }

  depends_on = [aws_s3_bucket_public_access_block.tower]
}
