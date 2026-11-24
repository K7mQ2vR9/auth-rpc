-- 2026-11-24 auth-rpc迁移前结构快照；目标库user_rpc。
-- 仅保存结构，不含账户记录、连接凭证或用户数据；不是可直接执行的回滚脚本。
-- 角色表迁移前记录数为0；其余三张表不在本次DDL修改范围。

CREATE TABLE `role_permission` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `role_name` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `permission_ids` json NOT NULL COMMENT '权限ID',
  `created_at` bigint DEFAULT NULL,
  `updated_at` bigint DEFAULT NULL,
  `deleted_at` bigint DEFAULT NULL,
  `business_code` varchar(10) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT '',
  `is_super_role` int NOT NULL DEFAULT '2' COMMENT '是否超级管理员，1=超级管理员，2=普通管理员，3=代理管理员',
  PRIMARY KEY (`id`) USING BTREE,
  KEY `idx_business` (`business_code`) USING BTREE
) ENGINE=InnoDB AUTO_INCREMENT=123 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci ROW_FORMAT=DYNAMIC;

CREATE TABLE `admin_info` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `account` varchar(50) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '账号',
  `name` varchar(50) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '账号名字',
  `password` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '密码',
  `password_sign` varchar(8) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT '' COMMENT '密码盐值',
  `role_id` bigint unsigned NOT NULL DEFAULT '0' COMMENT '角色ID',
  `status` int unsigned NOT NULL DEFAULT '1' COMMENT '1=正常；2=异常',
  `business_code` varchar(10) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '用户所属的商户编码',
  `created_at` bigint DEFAULT NULL COMMENT '创建时间',
  `updated_at` bigint DEFAULT NULL COMMENT '更新时间',
  `deleted_at` bigint DEFAULT NULL,
  `game_user_id` bigint NOT NULL DEFAULT '0' COMMENT 'C端用户ID',
  `account_type` int NOT NULL DEFAULT '0' COMMENT '账号类型 10=平台超级管理员，11=平台管理员，20=商户超级管理员，30=站点管理员,40=代理管理员',
  `business_codes` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '站点普通管理员所管理的站点编码',
  `google_code` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT 'googleCode',
  `is_google` int NOT NULL DEFAULT '1' COMMENT '1=需要，2=不需要',
  `login_err_times` int NOT NULL DEFAULT '0' COMMENT '登录的错误次数',
  `second_verify` int NOT NULL DEFAULT '1' COMMENT '是否开启二次验证，1=不开启，2=google code,3=邮箱，4=手机,5=密保',
  `real_email` varchar(100) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL DEFAULT '' COMMENT '邮箱',
  `verify_answer` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '问答密保',
  `ip_whitelist` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL COMMENT 'IP白名单',
  `second_verify_second` int NOT NULL DEFAULT '1' COMMENT '是否强制开启密保 1=不强制 2=强制',
  PRIMARY KEY (`id`) USING BTREE,
  UNIQUE KEY `uniq_account` (`account`,`deleted_at`) USING BTREE
) ENGINE=InnoDB AUTO_INCREMENT=1669 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci ROW_FORMAT=DYNAMIC;

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

CREATE TABLE `permission_menu` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `permission_name` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `menu_ids` json DEFAULT NULL,
  `created_at` bigint DEFAULT NULL,
  `updated_at` bigint DEFAULT NULL,
  `deleted_at` bigint DEFAULT NULL,
  `business_code` varchar(10) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT '',
  PRIMARY KEY (`id`) USING BTREE
) ENGINE=InnoDB AUTO_INCREMENT=112 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci ROW_FORMAT=DYNAMIC;
