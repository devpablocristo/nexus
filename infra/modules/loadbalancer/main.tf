locals {
  enable_https = trimspace(var.certificate_arn) != ""
  listener_arn = local.enable_https ? aws_lb_listener.https[0].arn : aws_lb_listener.http_forward[0].arn
}

resource "aws_lb" "main" {
  name               = "${var.environment}-nexus-alb"
  internal           = false
  load_balancer_type = "application"
  security_groups    = [var.sg_alb_id]
  subnets            = var.public_subnet_ids

  enable_deletion_protection = true

  tags = {
    Name = "${var.environment}-nexus-alb"
  }
}

resource "aws_lb_target_group" "core" {
  name        = "${var.environment}-nexus-core"
  port        = 8080
  protocol    = "HTTP"
  target_type = "ip"
  vpc_id      = var.vpc_id

  health_check {
    enabled             = true
    path                = "/readyz"
    matcher             = "200-399"
    healthy_threshold   = 2
    unhealthy_threshold = 3
    timeout             = 5
    interval            = 15
  }
}

resource "aws_lb_target_group" "saas" {
  name        = "${var.environment}-nexus-saas"
  port        = 8082
  protocol    = "HTTP"
  target_type = "ip"
  vpc_id      = var.vpc_id

  health_check {
    enabled             = true
    path                = "/health"
    matcher             = "200-399"
    healthy_threshold   = 2
    unhealthy_threshold = 3
    timeout             = 5
    interval            = 15
  }
}

resource "aws_lb_listener" "http_redirect" {
  count             = local.enable_https ? 1 : 0
  load_balancer_arn = aws_lb.main.arn
  port              = 80
  protocol          = "HTTP"

  default_action {
    type = "redirect"

    redirect {
      port        = "443"
      protocol    = "HTTPS"
      status_code = "HTTP_301"
    }
  }
}

resource "aws_lb_listener" "http_forward" {
  count             = local.enable_https ? 0 : 1
  load_balancer_arn = aws_lb.main.arn
  port              = 80
  protocol          = "HTTP"

  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.core.arn
  }
}

resource "aws_lb_listener" "https" {
  count             = local.enable_https ? 1 : 0
  load_balancer_arn = aws_lb.main.arn
  port              = 443
  protocol          = "HTTPS"
  ssl_policy        = "ELBSecurityPolicy-TLS13-1-2-2021-06"
  certificate_arn   = var.certificate_arn

  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.core.arn
  }
}

# Core API and gateway surfaces
resource "aws_lb_listener_rule" "core_api_a" {
  listener_arn = local.listener_arn
  priority     = 10

  action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.core.arn
  }

  condition {
    path_pattern {
      values = ["/v1/run*", "/v1/tools*", "/v1/policies*", "/v1/audit*", "/v1/secrets*"]
    }
  }
}

resource "aws_lb_listener_rule" "core_api_b" {
  listener_arn = local.listener_arn
  priority     = 11

  action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.core.arn
  }

  condition {
    path_pattern {
      values = ["/v1/approvals*", "/mcp", "/a2a/*"]
    }
  }
}

# SaaS surfaces
resource "aws_lb_listener_rule" "saas_api_a" {
  listener_arn = local.listener_arn
  priority     = 20

  action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.saas.arn
  }

  condition {
    path_pattern {
      values = ["/v1/admin*", "/v1/billing*", "/v1/incidents*", "/v1/notifications*", "/v1/users*"]
    }
  }
}

resource "aws_lb_listener_rule" "saas_api_b" {
  listener_arn = local.listener_arn
  priority     = 21

  action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.saas.arn
  }

  condition {
    path_pattern {
      values = ["/v1/events*", "/v1/actions*", "/v1/alert-rules*", "/v1/sessions*", "/v1/policy-proposals*"]
    }
  }
}

resource "aws_lb_listener_rule" "saas_api_c" {
  listener_arn = local.listener_arn
  priority     = 22

  action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.saas.arn
  }

  condition {
    path_pattern {
      values = ["/v1/assistant*", "/v1/orgs*", "/v1/auth/*", "/v1/webhooks/*", "/internal/*"]
    }
  }
}

# Core docs and health fallback
resource "aws_lb_listener_rule" "core_docs" {
  listener_arn = local.listener_arn
  priority     = 30

  action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.core.arn
  }

  condition {
    path_pattern {
      values = ["/openapi.yaml", "/docs", "/healthz", "/readyz"]
    }
  }
}
