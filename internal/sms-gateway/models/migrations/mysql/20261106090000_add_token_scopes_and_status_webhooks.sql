-- +goose Up
-- +goose StatementBegin
ALTER TABLE `tokens` ADD COLUMN `scopes` varchar(512) NULL AFTER `revoked_at`;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TABLE `status_webhooks` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `ext_id` varchar(36) NOT NULL,
    `user_id` varchar(32) NOT NULL,
    `url` varchar(256) NOT NULL,
    `secret` varchar(64) NOT NULL,
    `events` varchar(256) NOT NULL,
    `disabled` tinyint(1) NOT NULL DEFAULT 0,
    `failure_count` int NOT NULL DEFAULT 0,
    `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    `updated_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    `deleted_at` datetime(3) NULL,
    PRIMARY KEY (`id`),
    UNIQUE INDEX `unq_status_webhooks_user_extid` (`user_id`, `ext_id`),
    CONSTRAINT `fk_status_webhooks_user` FOREIGN KEY (`user_id`) REFERENCES `users`(`id`) ON DELETE CASCADE
) ENGINE = InnoDB;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS `status_webhooks`;
-- +goose StatementEnd
-- +goose StatementBegin
ALTER TABLE `tokens` DROP COLUMN `scopes`;
-- +goose StatementEnd
