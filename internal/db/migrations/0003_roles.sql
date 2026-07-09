CREATE TABLE roles (
    id uuid PRIMARY KEY,
    tenant_id text NOT NULL,
    name text NOT NULL,
    created timestamptz NOT NULL DEFAULT now(),
    modified timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, name),
    UNIQUE (tenant_id, id)
);

CREATE TABLE role_permissions (
    role_id uuid NOT NULL REFERENCES roles (id) ON DELETE CASCADE,
    resource_type_id uuid NOT NULL,
    action text NOT NULL,
    PRIMARY KEY (role_id, resource_type_id, action),
    FOREIGN KEY (resource_type_id, action) REFERENCES resource_type_actions (resource_type_id, action) ON DELETE RESTRICT
);

CREATE INDEX role_permissions_grant_idx ON role_permissions (resource_type_id, action);
