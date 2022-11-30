
-- +migrate Up
CREATE TABLE IF NOT EXISTS `be-position`.`position`
(
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT 'id',
    `member_id` BIGINT UNSIGNED NOT NULL COMMENT '會員id',
    `product_type` TINYINT(4) NOT NULL COMMENT '產品類別  1:stock, 2:crypto, 3:forex, 4:futures',
    `exchange_code` VARCHAR(32) NOT NULL COMMENT '交易所代號',
    `product_code` VARCHAR(32) NOT NULL COMMENT '產品代號',
    `trade_type` TINYINT(4) NOT NULL COMMENT '買賣類別 1:買 2:賣',
    `position_status` TINYINT(4) NOT NULL COMMENT '倉位狀態 1:開倉 2:關倉',
    `process_state` TINYINT(4) NOT NULL COMMENT '處理狀態 1:開倉 2:等待關倉 3:關倉',
    `amount` DECIMAL(19,4) NOT NULL COMMENT '倉位數量',
    `unit_price` DECIMAL(19,4) NOT NULL COMMENT '成交單價',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '創建時間',
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新時間',

    PRIMARY KEY (`id`),
    UNIQUE INDEX (`position_status`,`member_id`,`exchange_code`,`product_code`,`trade_type`,`created_at`)
) AUTO_INCREMENT=1 CHARSET=`utf8mb4` COLLATE=`utf8mb4_general_ci` COMMENT '倉位';


-- +migrate Down
SET FOREIGN_KEY_CHECKS=0;
DROP TABLE IF EXISTS `position`;
