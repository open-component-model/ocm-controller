package main

deny[msg] {
  not input.replicas

  msg := "Replicas must be set"
}

deny[msg] {
  input.replicas != 1

  msg := "Replicas must equal 1"
}
