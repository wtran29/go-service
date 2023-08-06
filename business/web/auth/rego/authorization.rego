package service.rego

default allowAny = false
default allowOnlyUser = false
default allowOnlyAdmin = false
default allowAdminOrSubject = false

roleUser := "USER"
roleAdmin := "ADMIN"
roleAll := {roleAdmin, roleUser}

allowAny {
	roles_from_claims := {role | role := input.Roles[_]}
	input_roles := roleAll & roles_from_claims
	count(input_roles) > 0
}

allowOnlyUser {
	roles_from_claims := {role | role := input.Roles[_]}
	input_user := {roleUser} & roles_from_claims
	count(input_user) > 0
}

allowOnlyAdmin {
	roles_from_claims := {role | role := input.Roles[_]}
	input_admin := {roleAdmin} & roles_from_claims
	count(input_admin) > 0
}

allowAdminOrSubject {
    roles_from_claims := {role | role := input.Roles[_]}
    input_admin := {roleAdmin} & roles_from_claims
    count(input_admin) > 0
} else {
    roles_from_claims := {role | role := input.Roles[_]}
    input_user := {roleUser} & roles_from_claims
    count(input_user) > 0
    input.UserID == input.Subject
}