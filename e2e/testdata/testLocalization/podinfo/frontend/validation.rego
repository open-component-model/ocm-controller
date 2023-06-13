package main

deny[msg] {
  not input.message

  msg := "Message is required"
}

deny[msg] {
  allowed_colors = ["red", "blue", "green", "yellow"]

  input.color != any(allowed_colors)

  msg := sprintf("Color must be one of: %v", [allowed_colors])
}

deny[msg] {
  not input.replicas

  msg := "Replicas must be set"
}

deny[msg] {
  input.replicas > 2

  msg := "Replicas must be less than 2"
}

deny[msg] {
  input.replicas == 0

  msg := "Replicas must not be zero"
}
