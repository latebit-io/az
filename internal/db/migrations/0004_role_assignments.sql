CREATE TABLE role_assignments (
    tenant_id text NOT NULL,
    subject text NOT NULL,
    role_id uuid NOT NULL REFERENCES roles (id) ON DELETE CASCADE,
    created timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, subject, role_id)
);

CREATE INDEX role_assignments_role_idx ON role_assignments (role_id);
