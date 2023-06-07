package main

deny[msg] {
  not input.replicas

  msg := "Replicas must be set"
}

deny[msg] {
  input.replicas == 0

  msg := "Replicas must not be zero"
}

deny[msg] {
  not input.replicas < 4

  msg := "Replicas must be less than 4"
}

deny[msg] {
  not input.cacheAddr

  msg := "Cache address is required"
}

deny[msg] {
  not regex.match(`tcp://[a-z.-]+:\d+`,input.cacheAddr)

  msg := "Cache address is not valid"
}
