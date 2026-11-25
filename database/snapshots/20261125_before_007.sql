-- 2026-11-25 user_rpc before migration007.
-- Six permission tables empty: schema and exact counts cover the empty data state.
-- No records, connection information or credentials; not an executable rollback script.

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
  `site_super_business_code` varchar(10) COLLATE utf8mb4_unicode_ci GENERATED ALWAYS AS ((case when (`account_type` = 20) then `business_code` else NULL end)) STORED COMMENT '本站超级管理员20唯一槽位，软删除不释放，其他身份NULL',
  PRIMARY KEY (`id`) USING BTREE,
  UNIQUE KEY `uniq_account` (`account`,`deleted_at`) USING BTREE,
  UNIQUE KEY `uniq_site_super` (`site_super_business_code`),
  CONSTRAINT `chk_site_super_scope` CHECK (((`account_type` <> 20) or ((`business_code` is not null) and (char_length(trim(`business_code`)) > 0) and (char_length(`business_code`) <= 10) and (char_length(`business_code`) = char_length(trim(`business_code`))) and (`business_code` <> _utf8mb4'99999999'))))
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
  `template_type` tinyint unsigned NOT NULL COMMENT '模板类型：1平台默认、2站点默认、3站点普通；身份和模板类型分别管理',
  `version` bigint unsigned NOT NULL DEFAULT '1' COMMENT '模板版本；首次1，修改及软删除时由业务按旧版本比较并递增，不参与权限计算',
  `created_at` bigint DEFAULT NULL,
  `updated_at` bigint DEFAULT NULL,
  `deleted_at` bigint DEFAULT NULL,
  `business_code` varchar(10) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT '',
  `root_business_code` varchar(10) COLLATE utf8mb4_unicode_ci GENERATED ALWAYS AS ((case when (`template_type` = 2) then `business_code` else NULL end)) STORED COMMENT '站点默认唯一槽位，类型2取业务编码，其余NULL，软删除不释放',
  `platform_default_slot` tinyint GENERATED ALWAYS AS ((case when (`template_type` = 1) then 1 else NULL end)) STORED COMMENT '平台默认唯一槽位，类型1固定1，其余NULL，软删除不释放',
  PRIMARY KEY (`id`) USING BTREE,
  UNIQUE KEY `uniq_site_root` (`root_business_code`),
  UNIQUE KEY `uniq_platform_default` (`platform_default_slot`),
  KEY `idx_business` (`business_code`) USING BTREE,
  CONSTRAINT `chk_role_template_business` CHECK (((`business_code` is not null) and (char_length(trim(`business_code`)) > 0))),
  CONSTRAINT `chk_role_template_scope` CHECK ((((`template_type` = 1) and (`business_code` = _utf8mb4'99999999')) or ((`template_type` in (2,3)) and (`business_code` <> _utf8mb4'99999999')))),
  CONSTRAINT `chk_role_template_type` CHECK ((`template_type` in (1,2,3))),
  CONSTRAINT `chk_role_template_version` CHECK ((`version` between 1 and 9223372036854775807))
) ENGINE=InnoDB AUTO_INCREMENT=123 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci ROW_FORMAT=DYNAMIC COMMENT='平台默认、站点默认和站点普通权限模板；用户独立授权，不绑定操作者';

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

-- permission_audit; exact row count: 0
CREATE TABLE `permission_audit` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '权限审计主键，应用限制在int64正数范围',
  `operation` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '服务端定义的操作名，如create_site_root_template，不接受请求体指定',
  `operator_id` bigint unsigned NOT NULL COMMENT '可信上下文对应的操作者ID，事务内查询管理员后记录',
  `operator_account_type` int NOT NULL COMMENT '操作时账号类型快照，沿用10、11、20、30、40现有定义',
  `operator_business_code` varchar(10) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '操作时管理员归属编码，与目标站点分别保存',
  `business_code` varchar(10) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '本次权限变更的目标站点编码',
  `target_type` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '目标类型：site_root_template根模板，user_permission用户独立授权',
  `target_id` bigint unsigned NOT NULL COMMENT '根模板目标使用模板ID，用户授权目标使用用户ID',
  `before_menu_ids` text COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '变更前的规范化具体权限ID逗号集合，创建前为空字符串',
  `after_menu_ids` text COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '变更后的规范化具体权限ID逗号集合，空字符串表示无权限',
  `before_version` bigint unsigned NOT NULL COMMENT '变更前目标版本，创建前为0',
  `after_version` bigint unsigned NOT NULL COMMENT '变更后目标版本，首次创建为1，其他更新递增1，复制记录用户授权版本',
  `source_template_id` bigint unsigned NOT NULL DEFAULT '0' COMMENT '模板复制来源ID，非复制操作为0，不参与实际授权',
  `source_template_version` bigint unsigned NOT NULL DEFAULT '0' COMMENT '模板复制来源版本，非复制操作为0',
  `trace_id` char(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '当前链路32位小写十六进制Trace ID，复用现有链路，不另生成',
  `created_at` bigint NOT NULL DEFAULT '0' COMMENT '记录时间，Unix秒，由应用模型填写',
  PRIMARY KEY (`id`),
  KEY `idx_audit_business_created` (`business_code`,`created_at`,`id`),
  KEY `idx_audit_operator_created` (`operator_id`,`created_at`,`id`),
  KEY `idx_audit_trace_id` (`trace_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='权限成功变更审计，与业务同事务保存快照，不存密码、Token或密钥，不做默认自动清理';

