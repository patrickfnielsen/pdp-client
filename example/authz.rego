package authz

import future.keywords.contains
import future.keywords.if
import future.keywords.in

default allow := false

mapping := [
	{
		"group": "Admins",
		"permissions": [
			"app.create",
			"app.delete",
		],
	},
	{
		"group": "Users",
		"permissions": ["app.view"],
	},
]

# user permission mapping
permissions contains perms if {
	some user_group in input.user.groups
	some x in mapping
	some perms in x.permissions
	x.group == user_group
}

# user group mapping
user_has_group contains group if {
	some group in input.user.groups
}

user_is_admin if user_has_group["Admins"]
user_has_permission if permissions[input.permission]


allow if user_is_admin
allow if user_has_permission

# permission reason
reasons contains "has-permission" if {
	allow
	user_has_permission
}

reasons contains "no-permission" if {
	not allow
	not user_has_permission
}

# admin reason
reasons contains "is-admin" if {
	allow
	user_is_admin
}

decision := {
	"allow": allow,
    "reasons": reasons
}