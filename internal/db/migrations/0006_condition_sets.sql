CREATE TABLE condition_sets (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id text NOT NULL,
    key text NOT NULL,
    name text NOT NULL,
    description text NOT NULL DEFAULT '',
    type text NOT NULL CHECK (type IN ('subject', 'resource')),
    resource_type text NOT NULL DEFAULT '',
    conditions jsonb NOT NULL,
    created timestamptz NOT NULL DEFAULT now(),
    modified timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, key)
);

CREATE TABLE condition_set_rules (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id text NOT NULL,
    subject_set text NOT NULL,
    resource text NOT NULL,
    action text NOT NULL,
    resource_set text NOT NULL,
    created timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, subject_set, resource, action, resource_set),
    FOREIGN KEY (tenant_id, subject_set) REFERENCES condition_sets (tenant_id, key) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, resource_set) REFERENCES condition_sets (tenant_id, key) ON DELETE RESTRICT
);

CREATE INDEX condition_set_rules_check_idx ON condition_set_rules (tenant_id, resource, action);
