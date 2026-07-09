CREATE TABLE roles (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id text NOT NULL,
    key text NOT NULL,
    name text NOT NULL,
    description text NOT NULL DEFAULT '',
    created timestamptz NOT NULL DEFAULT now(),
    modified timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, key)
);

CREATE TABLE role_permissions (
    tenant_id text NOT NULL,
    role text NOT NULL,
    resource text NOT NULL,
    action text NOT NULL,
    PRIMARY KEY (tenant_id, role, resource, action),
    FOREIGN KEY (tenant_id, role) REFERENCES roles (tenant_id, key) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id, resource, action) REFERENCES resource_type_actions (tenant_id, resource, action) ON DELETE RESTRICT
);

CREATE INDEX role_permissions_grant_idx ON role_permissions (tenant_id, resource, action);
