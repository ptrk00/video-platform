-- +goose Up

INSERT INTO app_users (id, username, password) VALUES 
    (1, 'user1', '$2a$12$W3HwBfnl.RWRTELcnoZ7x.9Djh8.B2SCH/QhV81iTT68FTP9AQ8ce'),
    (2, 'user2', '$2a$12$W3HwBfnl.RWRTELcnoZ7x.9Djh8.B2SCH/QhV81iTT68FTP9AQ8ce')
;