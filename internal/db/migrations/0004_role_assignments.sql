CREATE TABLE role_assignments (
    tenant_id text NOT NULL,
    subject text NOT NULL,
    role_id uuid NOT NULL,
    created timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, subject, role_id),
    -- composite FK: an assignment can only reference a role owned by the
    -- same tenant
    FOREIGN KEY (tenant_id, role_id) REFERENCES roles (tenant_id, id) ON DELETE CASCADE
);

CREATE INDEX role_assignments_role_idx ON role_assignments (tenant_id, role_id);
