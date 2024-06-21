vault {
    address = "http://vault:8200"
    retry {
        num_retries = 5
    }
}


auto_auth {
    method {
        type = "token_file"
        config = {
            token_file_path = "/etc/vault/root_token"
        }
    }
}

template {
  source      = "/etc/vault/templates/minio_credentials.ctmpl"
  destination = "/vault/secrets/minio_credentials"
  perms       = "0644"
}
