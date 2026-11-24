-- user_rpc菜单服务字段迁移前结构快照，不含凭证或账户数据，不是直接回滚脚本。
CREATE TABLE `menu` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `menu_pid` bigint unsigned NOT NULL DEFAULT '0',
  `menu_name` varchar(100) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `menu_type` int unsigned NOT NULL DEFAULT '0' COMMENT '菜单类型 1=页面，2=按钮，3=接口',
  `path` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '请求路径，接口路径',
  `status` int unsigned NOT NULL DEFAULT '0' COMMENT '1=启用；2=禁用',
  `sort` int NOT NULL DEFAULT '1' COMMENT '排序，越小越前',
  `created_at` bigint DEFAULT NULL,
  `updated_at` bigint DEFAULT NULL,
  `deleted_at` bigint DEFAULT NULL,
  `business_code` varchar(10) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT '',
  PRIMARY KEY (`id`) USING BTREE,
  KEY `uniq` (`path`)
) ENGINE=InnoDB AUTO_INCREMENT=784 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci ROW_FORMAT=DYNAMIC;
