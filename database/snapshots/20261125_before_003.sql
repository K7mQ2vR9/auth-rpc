-- 2026-11-25 auth-rpc迁移003前结构快照；目标库user_rpc。
-- 仅保存role_permission结构，不含记录、连接凭证或密钥；不是可直接执行的回滚脚本。
-- 快照时role_permission包含软删除记录的精确COUNT为0。

CREATE TABLE `role_permission` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `role_name` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `menu_ids` text COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '英文逗号分隔的具体菜单ID；空字符串表示空模板，不再保存权限组ID或JSON数组',
  `created_at` bigint DEFAULT NULL,
  `updated_at` bigint DEFAULT NULL,
  `deleted_at` bigint DEFAULT NULL,
  `business_code` varchar(10) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT '',
  PRIMARY KEY (`id`) USING BTREE,
  KEY `idx_business` (`business_code`) USING BTREE
) ENGINE=InnoDB AUTO_INCREMENT=123 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci ROW_FORMAT=DYNAMIC COMMENT='角色权限模板；复制后用户权限独立，修改或删除模板不联动用户';
