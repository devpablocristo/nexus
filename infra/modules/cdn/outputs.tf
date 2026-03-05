output "bucket_name" {
  value = aws_s3_bucket.tower.bucket
}

output "bucket_arn" {
  value = aws_s3_bucket.tower.arn
}

output "cloudfront_distribution_id" {
  value = aws_cloudfront_distribution.tower.id
}

output "cloudfront_domain_name" {
  value = aws_cloudfront_distribution.tower.domain_name
}

output "cloudfront_hosted_zone_id" {
  value = aws_cloudfront_distribution.tower.hosted_zone_id
}
