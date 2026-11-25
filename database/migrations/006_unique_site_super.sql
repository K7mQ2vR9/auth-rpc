-- 站点超级管理员20唯一身份；2026-11-25已在user_rpc执行成功，禁止在该库重跑，不自动初始化账号。
-- 执行前核对005、admin_info实际结构，备份数据，排查重复20、空白/平台归属、异常软删除状态。
-- 新含义20=站点超级管理员，迁移不把已有身份自动改为其他类型，也不删除或合并重复账号。
-- 唯一槽位不含deleted_at，已删除20保留名额；替换/删除/恢复流程须另行讨论，不自动创建第二个20。
-- 索引保证每站点最多一位；每站点至少一位须由建站/账号初始化流程保证，auth-rpc当前没有账号创建或删除入口。
-- 本文件不创建数据库，不写用户/授权样本，DDL隐式提交，禁止重跑。

ALTER TABLE `admin_info`
  ADD COLUMN `site_super_business_code` VARCHAR(10)
    GENERATED ALWAYS AS (CASE WHEN `account_type` = 20 THEN `business_code` ELSE NULL END) STORED
    COMMENT '本站超级管理员20唯一槽位，软删除不释放，其他身份NULL',
  ADD UNIQUE KEY `uniq_site_super` (`site_super_business_code`),
  ADD CONSTRAINT `chk_site_super_scope` CHECK
    (`account_type` <> 20 OR (`business_code` IS NOT NULL AND
      CHAR_LENGTH(TRIM(`business_code`)) > 0 AND CHAR_LENGTH(`business_code`) <= 10 AND
      CHAR_LENGTH(`business_code`) = CHAR_LENGTH(TRIM(`business_code`)) AND `business_code` <> '99999999'));

-- 运行时及模板管理仍校验20正常、未删除、同站点且唯一；停用不会自动授予第二位超级管理员。
