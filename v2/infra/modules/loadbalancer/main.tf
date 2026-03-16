resource "aws_security_group" "this" {
  name        = "${var.name_prefix}-alb"
  description = "Security group for the Nexus public ALB."
  vpc_id      = var.vpc_id

  tags = merge(var.tags, {
    Name = "${var.name_prefix}-alb-sg"
  })
}

locals {
  ingress_rules = {
    for pair in flatten([
      for service_name, service in var.services : [
        for cidr in var.allowed_cidrs : {
          key  = "${service_name}-${service.listener_port}-${cidr}"
          cidr = cidr
          port = service.listener_port
        }
      ]
    ]) : pair.key => pair
  }
}

resource "aws_vpc_security_group_ingress_rule" "listeners" {
  for_each = local.ingress_rules

  security_group_id = aws_security_group.this.id
  cidr_ipv4         = each.value.cidr
  from_port         = each.value.port
  to_port           = each.value.port
  ip_protocol       = "tcp"
}

resource "aws_vpc_security_group_egress_rule" "all" {
  security_group_id = aws_security_group.this.id
  cidr_ipv4         = "0.0.0.0/0"
  ip_protocol       = "-1"
}

resource "aws_lb" "this" {
  name               = substr("${var.name_prefix}-alb", 0, 32)
  load_balancer_type = "application"
  internal           = false
  security_groups    = [aws_security_group.this.id]
  subnets            = var.public_subnet_ids

  tags = merge(var.tags, {
    Name = "${var.name_prefix}-alb"
  })
}

resource "aws_lb_target_group" "this" {
  for_each = var.services

  name        = substr(replace("${var.name_prefix}-${each.key}", "/[^a-zA-Z0-9-]/", "-"), 0, 32)
  port        = each.value.target_port
  protocol    = "HTTP"
  target_type = "ip"
  vpc_id      = var.vpc_id

  health_check {
    path                = each.value.health_check_path
    healthy_threshold   = 2
    unhealthy_threshold = 2
    interval            = 30
    timeout             = 5
    matcher             = "200"
  }

  tags = merge(var.tags, {
    Name = "${var.name_prefix}-${each.key}-tg"
  })
}

resource "aws_lb_listener" "this" {
  for_each = var.services

  load_balancer_arn = aws_lb.this.arn
  port              = each.value.listener_port
  protocol          = each.value.listener_protocol
  ssl_policy        = each.value.listener_protocol == "HTTPS" ? "ELBSecurityPolicy-TLS13-1-2-2021-06" : null
  certificate_arn   = each.value.listener_protocol == "HTTPS" ? each.value.certificate_arn : null

  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.this[each.key].arn
  }
}
