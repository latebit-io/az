CREATE TABLE resource_types (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id text NOT NULL,
    key text NOT NULL,
    created timestamptz NOT NULL DEFAULT now(),
    modified timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, key)
);

CREATE TABLE resource_type_actions (
    tenant_id text NOT NULL,
    resource text NOT NULL,
    action text NOT NULL,
    PRIMARY KEY (tenant_id, resource, action),
    FOREIGN KEY (tenant_id, resource) REFERENCES resource_types (tenant_id, key) ON DELETE CASCADE
);
