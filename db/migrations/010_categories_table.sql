CREATE TABLE categories (
    id varchar(36) NOT NULL,
    name varchar(255) NOT NULL,
    created_at datetime NOT NULL,
    updated_at datetime NOT NULL,
    max_amount decimal(20, 2),
    created_by varchar(36),
    CONSTRAINT pk_categories PRIMARY KEY(id),
    CONSTRAINT uq_categories_name UNIQUE(name, created_by),
    CONSTRAINT fk_categories_created_by_users_id FOREIGN KEY(created_by) REFERENCES users(id)
);