CREATE TABLE resource_types (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id text NOT NULL,
    key text NOT NULL,
    name text NOT NULL,
    description text NOT NULL DEFAULT '',
    actions text[] NOT NULL,
    attributes jsonb NOT NULL DEFAULT '[]',
    created timestamptz NOT NULL DEFAULT now(),
    modified timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, key)
);
