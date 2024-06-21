package authz

default allow = false

allow {
    input.username == "user1"
}