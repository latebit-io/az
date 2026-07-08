CREATE TABLE role_assignments (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id text NOT NULL,
    subject text NOT NULL,
    role text NOT NULL,
    created timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, subject, role),
    FOREIGN KEY (tenant_id, role) REFERENCES roles (tenant_id, key) ON DELETE CASCADE
);

CREATE INDEX role_assignments_subject_idx ON role_assignments (tenant_id, subject);
CREATE INDEX role_assignments_role_idx ON role_assignments (tenant_id, role);
