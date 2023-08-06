package service.rego

default auth = false

# This function decodes and verifies the JWT, it also makes sure that it hasn't expired etc. 
auth {
	jwt_valid
}

jwt_valid := valid {
	[valid, header, payload] := verify_jwt
}

verify_jwt := io.jwt.decode_verify(input.Token, {
        "cert": input.Key,
        "iss": input.ISS,
	}
)

