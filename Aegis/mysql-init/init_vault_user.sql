CREATE USER 'vault_admin'@'%' IDENTIFIED BY 'secure-vault-password';

GRANT ALL PRIVILEGES ON *.* TO 'vault_admin'@'%' WITH GRANT OPTION;

FLUSH PRIVILEGES;