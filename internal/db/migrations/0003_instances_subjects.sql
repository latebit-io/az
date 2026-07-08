CREATE TABLE resource_instances (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id text NOT NULL,
    resource_type text NOT NULL,
    key text NOT NULL,
    attributes jsonb NOT NULL DEFAULT '{}',
    created timestamptz NOT NULL DEFAULT now(),
    modified timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, resource_type, key),
    FOREIGN KEY (tenant_id, resource_type) REFERENCES resource_types (tenant_id, key) ON DELETE CASCADE
);

CREATE TABLE subjects (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id text NOT NULL,
    key text NOT NULL,
    email text NOT NULL DEFAULT '',
    attributes jsonb NOT NULL DEFAULT '{}',
    created timestamptz NOT NULL DEFAULT now(),
    modified timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, key)
);

CREATE INDEX subjects_tenant_email_idx ON subjects (tenant_id, email);
