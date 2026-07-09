CREATE TABLE resource_types (
    id uuid PRIMARY KEY,
    tenant_id text NOT NULL,
    name text NOT NULL,
    created timestamptz NOT NULL DEFAULT now(),
    modified timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, name)
);

CREATE TABLE resource_type_actions (
    resource_type_id uuid NOT NULL REFERENCES resource_types (id) ON DELETE CASCADE,
    action text NOT NULL,
    PRIMARY KEY (resource_type_id, action)
);
