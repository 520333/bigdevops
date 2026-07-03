package cache

//func (mc *MonitorCache) GeneratePrometheusRuleMainConfigYaml(ctx context.Context) {
//	pools, err := models.GetMonitorScrapePoolAll()
//	if err != nil {
//		mc.Sc.Logger.Error("[监控模块]扫描数据库中的采集池失败", zap.Error(err))
//		return
//	}
//	if len(pools) == 0 {
//		return
//	}
//	mainConfigMap := map[string]string{}
//	for _, pool := range pools {
//		pool := pool
//		// 1. 生成基础配置 (global, remote_write)
//		allConfig := mc.GeneratePrometheusMainConfigYamlOnePool(pool)
//
//		// 2. 生成并附加采集段 (scrape_configs) - 必须在 Marshal 之前！
//		baseScrapeConfigs := mc.GeneratePrometheusScrapeConfigYamlOnePool(pool)
//		if baseScrapeConfigs == nil {
//			continue
//		}
//		//scrapeConfigs := mc.GeneratePrometheusScrapeConfigYamlOnePool(pool)
//		//if scrapeConfigs == nil {
//		//	continue
//		//}
//
//		ipNum := len(pool.PrometheusInstances)
//
//		for index, ip := range pool.PrometheusInstances {
//			ip := ip
//			// 根据数量做hashmod的判断 大于0需要分片
//			//if ipNum > 0 {
//			//	scrapeConfigs = mc.HashModScrapeConfig(scrapeConfigs, ipNum, index)
//			//}
//			//allConfig.ScrapeConfigs = scrapeConfigs
//			//// 3. 所有内容组装完毕，执行序列化转为 YAML
//			//out, err := yaml.Marshal(allConfig)
//			//if err != nil {
//			//	mc.Sc.Logger.Error("[监控模块]根据采集池配置生成prometheus主配置文件错误", zap.Error(err), zap.String("采集池", pool.Name))
//			//	continue
//			//}
//			var currentScrapeConfigs []*ppc.ScrapeConfig
//			if ipNum > 1 {
//				currentScrapeConfigs = mc.HashModScrapeConfig(baseScrapeConfigs, ipNum, index)
//			} else {
//				currentScrapeConfigs = baseScrapeConfigs
//			}
//
//			allConfig.ScrapeConfigs = currentScrapeConfigs
//
//			// 3. 所有内容组装完毕，执行序列化转为 YAML
//			out, err := yaml.Marshal(allConfig)
//			if err != nil {
//				mc.Sc.Logger.Error("[监控模块]根据采集池配置生成prometheus主配置文件错误", zap.Error(err), zap.String("采集池", pool.Name))
//				continue
//			}
//			outStr := string(out)
//			// 重新查一遍当前 pool 下的任务（因为 allConfig.ScrapeConfigs 里的顺序和查询出的顺序一致）
//			scrapeJobs, _ := models.GetMonitorScrapeJobByPoolId(pool.ID)
//			for _, job := range scrapeJobs {
//				if job.BearerToken != "" {
//					// Kubernetes SD 配置中通常有两处 token (外部 HTTPClientConfig 和 SD 内部的 HTTPClientConfig)
//					replaceCount := 1
//					if job.ServiceDiscoveryType == common.MONITOR_SCRAPE_JOB_SD_TYPE_K8S {
//						replaceCount = 2
//					}
//
//					// 将被安全屏蔽的 <secret> 按顺序替换回真实的 Token，并加上双引号防止特殊字符导致 YAML 解析报错
//					outStr = strings.Replace(outStr, "bearer_token: <secret>", fmt.Sprintf(`bearer_token: "%s"`, job.BearerToken), replaceCount)
//				}
//			}
//
//			mc.Sc.Logger.Info("[监控模块]根据采集池配置生成prometheus主配置文件成功", zap.String("采集池", pool.Name), zap.Any("配置", outStr))
//			//fileName := fmt.Sprintf("pool_%v.yml", pool.Name)
//			//_ = os.WriteFile(fileName, out, 0666)
//			//mainConfigMap[ip] = string(out)
//			mainConfigMap[ip] = outStr
//		}
//	}
//
//	mc.Lock()
//	mc.PrometheusMainConfigMap = mainConfigMap
//	mc.Unlock()
//
//}
