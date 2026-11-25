-- 三类权限模板增量结构；2026-11-25已在user_rpc执行成功，禁止应用启动自动迁移或在该库重跑。
-- 前置：已完成003/004，仍为旧类型1站点根及2自定义；暂停相关写入口并备份结构和数据。
-- 核对旧记录仅类型1/2、均非平台归属、站点根唯一，且目标新列/索引/约束不存在。
-- 新类型1平台默认：全平台唯一、平台归属99999999，由10维护真实权限集合，超级管理员自身全权限不由此记录授予。
-- 新类型2站点默认：每站点唯一、仅11维护、不能超过平台默认；新类型3本站普通模板全部操作仅本站唯一20。
-- 三阶段DDL隐式提交；任一步失败按实际阶段处理，不从头盲目重复转换。

ALTER TABLE `role_permission`
  DROP CHECK `chk_role_template_type`,
  DROP INDEX `uniq_site_root`;

-- 仅重编号旧类型，保留模板ID、归属、权限CSV、版本及时间，含软删除记录。
UPDATE `role_permission`
SET `template_type` = CASE `template_type` WHEN 1 THEN 2 WHEN 2 THEN 3 END
WHERE `template_type` IN (1, 2);

ALTER TABLE `role_permission`
  MODIFY COLUMN `template_type` TINYINT UNSIGNED NOT NULL
    COMMENT '模板类型：1平台默认、2站点默认、3站点普通；身份和模板类型分别管理',
  MODIFY COLUMN `root_business_code` VARCHAR(10)
    GENERATED ALWAYS AS (CASE WHEN `template_type` = 2 THEN `business_code` ELSE NULL END) STORED
    COMMENT '站点默认唯一槽位，类型2取业务编码，其余NULL，软删除不释放',
  ADD UNIQUE KEY `uniq_site_root` (`root_business_code`),
  ADD COLUMN `platform_default_slot` TINYINT
    GENERATED ALWAYS AS (CASE WHEN `template_type` = 1 THEN 1 ELSE NULL END) STORED
    COMMENT '平台默认唯一槽位，类型1固定1，其余NULL，软删除不释放',
  ADD UNIQUE KEY `uniq_platform_default` (`platform_default_slot`),
  ADD CONSTRAINT `chk_role_template_type` CHECK (`template_type` IN (1, 2, 3)),
  ADD CONSTRAINT `chk_role_template_scope` CHECK
    ((`template_type` = 1 AND `business_code` = '99999999') OR
     (`template_type` IN (2, 3) AND `business_code` <> '99999999')),
  COMMENT = '平台默认、站点默认和站点普通权限模板；用户独立授权，不绑定操作者';

-- 不自动创建平台默认、站点默认、用户或权限数据。新源码须在本迁移核对完成后部署。
