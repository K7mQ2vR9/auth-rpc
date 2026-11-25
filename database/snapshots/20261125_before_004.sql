-- 2026-11-25 migration004 preflight: user_rpc existing permission tables.
-- Schema only; no account records, connection information, credentials or keys.
-- permission_audit did not exist. This is not an executable rollback script.

-- admin_info; exact row count: 0
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

-- admin_permission; exact row count: 0
CREATE TABLE `admin_permission` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '用户权限记录主键；应用使用int64时限制在MaxInt64以内',
  `user_id` bigint unsigned NOT NULL COMMENT '管理员ID，对应admin_info.id；一个用户只保存一条记录，不绑定角色',
  `menu_ids` text COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '英文逗号分隔的具体菜单ID；空字符串表示无权限，写入时显式传入；应用负责校验、去重和排序',
  `business_code` varchar(10) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '权限记录的业务归属；默认空，写入时由业务显式确定',
  `version` bigint unsigned NOT NULL DEFAULT '1' COMMENT '权限记录版本；首次保存为1，后续覆盖保存按版本校验并递增，不参与权限计算',
  `created_at` bigint NOT NULL DEFAULT '0' COMMENT '创建时间，Unix秒级时间戳；默认0，首次保存时由应用写入',
  `updated_at` bigint NOT NULL DEFAULT '0' COMMENT '最后修改时间，Unix秒级时间戳；默认0，保存时由应用写入',
  `deleted_at` bigint DEFAULT NULL COMMENT '软删除时间，Unix秒级时间戳；NULL表示未删除；恢复时复用原用户记录',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uniq_user_id` (`user_id`),
  KEY `idx_business_code` (`business_code`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='管理员独立菜单权限；模板仅作覆盖复制来源，后续模板变更不联动用户';

-- role_permission; exact row count: 0
CREATE TABLE `role_permission` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `role_name` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `menu_ids` text COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '英文逗号分隔的具体菜单ID；空字符串表示空模板，不再保存权限组ID或JSON数组',
  `template_type` tinyint unsigned NOT NULL COMMENT '模板用途：1站点根模板、2自定义模板；无默认类型，写入必须明确指定，普通CRUD仅允许2',
  `version` bigint unsigned NOT NULL DEFAULT '1' COMMENT '模板版本；首次1，修改及软删除时由业务按旧版本比较并递增，不参与权限计算',
  `created_at` bigint DEFAULT NULL,
  `updated_at` bigint DEFAULT NULL,
  `deleted_at` bigint DEFAULT NULL,
  `business_code` varchar(10) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT '',
  `root_business_code` varchar(10) COLLATE utf8mb4_unicode_ci GENERATED ALWAYS AS ((case when (`template_type` = 1) then `business_code` else NULL end)) STORED COMMENT '根模板唯一槽位，仅根模板生成站点编码；自定义模板为NULL，软删除不释放槽位',
  PRIMARY KEY (`id`) USING BTREE,
  UNIQUE KEY `uniq_site_root` (`root_business_code`),
  KEY `idx_business` (`business_code`) USING BTREE,
  CONSTRAINT `chk_role_template_business` CHECK (((`business_code` is not null) and (char_length(trim(`business_code`)) > 0))),
  CONSTRAINT `chk_role_template_type` CHECK ((`template_type` in (1,2))),
  CONSTRAINT `chk_role_template_version` CHECK ((`version` between 1 and 9223372036854775807))
) ENGINE=InnoDB AUTO_INCREMENT=123 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci ROW_FORMAT=DYNAMIC COMMENT='站点根权限与自定义权限模板；保存具体菜单ID，用户独立授权，不继承权限组或绑定角色';

-- menu; exact row count: 0
CREATE TABLE `menu` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `menu_pid` bigint unsigned NOT NULL DEFAULT '0',
  `menu_name` varchar(100) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `menu_type` int unsigned NOT NULL DEFAULT '0' COMMENT '菜单类型 1=页面，2=按钮，3=接口',
  `path` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '请求路径，接口路径',
  `service_name` varchar(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL COMMENT '接口所属API服务名，如admin-api；增改及权限校验必填，区分大小写，不提供默认服务或空值通配',
  `status` int unsigned NOT NULL DEFAULT '0' COMMENT '1=启用；2=禁用',
  `sort` int NOT NULL DEFAULT '1' COMMENT '排序，越小越前',
  `created_at` bigint DEFAULT NULL,
  `updated_at` bigint DEFAULT NULL,
  `deleted_at` bigint DEFAULT NULL,
  `business_code` varchar(10) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT '',
  PRIMARY KEY (`id`) USING BTREE,
  KEY `uniq` (`path`),
  KEY `idx_service_path` (`business_code`,`service_name`,`path`)
) ENGINE=InnoDB AUTO_INCREMENT=784 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci ROW_FORMAT=DYNAMIC;

-- menu_group; exact row count: 0
CREATE TABLE `menu_group` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `permission_name` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `menu_ids` json DEFAULT NULL,
  `created_at` bigint DEFAULT NULL,
  `updated_at` bigint DEFAULT NULL,
  `deleted_at` bigint DEFAULT NULL,
  `business_code` varchar(10) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT '',
  PRIMARY KEY (`id`) USING BTREE
) ENGINE=InnoDB AUTO_INCREMENT=112 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci ROW_FORMAT=DYNAMIC;
