-- +goose Up
-- +goose StatementBegin
CREATE TABLE `subscriptions` (
    `user_id` varchar(32) NOT NULL,
    `current_period_end` datetime(3) NOT NULL,
    `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    `updated_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (`user_id`),
    CONSTRAINT `fk_subscriptions_user` FOREIGN KEY (`user_id`) REFERENCES `users`(`id`) ON DELETE CASCADE
) ENGINE = InnoDB;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TABLE `payments` (
    `id` char(21) NOT NULL,
    `user_id` varchar(32) NOT NULL,
    `pesepay_reference_number` varchar(64) NOT NULL,
    `amount` decimal(10,2) NOT NULL,
    `currency` varchar(8) NOT NULL,
    `status` enum('initiated','paid','failed') NOT NULL DEFAULT 'initiated',
    `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    `updated_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (`id`),
    UNIQUE INDEX `unq_payments_reference` (`pesepay_reference_number`),
    INDEX `idx_payments_user_id` (`user_id`),
    CONSTRAINT `fk_payments_user` FOREIGN KEY (`user_id`) REFERENCES `users`(`id`) ON DELETE CASCADE
) ENGINE = InnoDB;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS `payments`;
-- +goose StatementEnd
-- +goose StatementBegin
DROP TABLE IF EXISTS `subscriptions`;
-- +goose StatementEnd
