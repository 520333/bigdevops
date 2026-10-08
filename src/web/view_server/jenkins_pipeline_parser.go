package view_server

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"regexp"
	"strings"
)

// ParsedJenkinsParam 表示从 Pipeline parameters 块中解析出的单个参数
type ParsedJenkinsParam struct {
	Type                 string // string, booleanParam, choice, text, password, reactiveChoice, activeChoice, cascadeChoice
	Name                 string
	Description          string
	DefaultValue         string
	Trim                 bool
	Choices              []string
	ChoiceType           string
	RandomName           string
	ReferencedParameters string
	Filterable           bool
	GroovyScript         string
	Sandbox              bool
}

// escapeXmlText 转义 XML 特殊字符
func escapeXmlText(s string) string {
	var buf bytes.Buffer
	xml.EscapeText(&buf, []byte(s))
	return buf.String()
}

// extractPipelineBlock 提取诸如 parameters { ... } 或 options { ... } 的大括号闭合内容
func extractPipelineBlock(script string, blockName string) string {
	pattern := regexp.MustCompile(`(?m)\b` + regexp.QuoteMeta(blockName) + `\s*\{`)
	loc := pattern.FindStringIndex(script)
	if loc == nil {
		return ""
	}
	start := loc[1]
	depth := 1
	inSingleQuote := false
	inDoubleQuote := false
	inTripleSingle := false
	inTripleDouble := false

	chars := []rune(script[start:])
	endOffset := -1

	for i := 0; i < len(chars); i++ {
		ch := chars[i]

		// 检查三引号
		if !inSingleQuote && !inDoubleQuote {
			if !inTripleSingle && !inTripleDouble && i+2 < len(chars) && chars[i] == '\'' && chars[i+1] == '\'' && chars[i+2] == '\'' {
				inTripleSingle = true
				i += 2
				continue
			} else if inTripleSingle && i+2 < len(chars) && chars[i] == '\'' && chars[i+1] == '\'' && chars[i+2] == '\'' {
				inTripleSingle = false
				i += 2
				continue
			} else if !inTripleSingle && !inTripleDouble && i+2 < len(chars) && chars[i] == '"' && chars[i+1] == '"' && chars[i+2] == '"' {
				inTripleDouble = true
				i += 2
				continue
			} else if inTripleDouble && i+2 < len(chars) && chars[i] == '"' && chars[i+1] == '"' && chars[i+2] == '"' {
				inTripleDouble = false
				i += 2
				continue
			}
		}

		if inTripleSingle || inTripleDouble {
			continue
		}

		// 单双引号
		if ch == '\\' && (inSingleQuote || inDoubleQuote) {
			i++ // 跳过转义字符
			continue
		}
		if ch == '\'' && !inDoubleQuote {
			inSingleQuote = !inSingleQuote
			continue
		}
		if ch == '"' && !inSingleQuote {
			inDoubleQuote = !inDoubleQuote
			continue
		}

		if inSingleQuote || inDoubleQuote {
			continue
		}

		if ch == '{' {
			depth++
		} else if ch == '}' {
			depth--
			if depth == 0 {
				endOffset = i
				break
			}
		}
	}

	if endOffset == -1 {
		return ""
	}
	return string(chars[:endOffset])
}

// cleanGroovyQuotes 去除首尾的各种 Groovy 引号并清理
func cleanGroovyQuotes(val string) string {
	val = strings.TrimSpace(val)
	if strings.HasPrefix(val, "'''") && strings.HasSuffix(val, "'''") && len(val) >= 6 {
		return val[3 : len(val)-3]
	}
	if strings.HasPrefix(val, `"""`) && strings.HasSuffix(val, `"""`) && len(val) >= 6 {
		return val[3 : len(val)-3]
	}
	if (strings.HasPrefix(val, "'") && strings.HasSuffix(val, "'")) ||
		(strings.HasPrefix(val, `"`) && strings.HasSuffix(val, `"`)) {
		if len(val) >= 2 {
			return val[1 : len(val)-1]
		}
	}
	return val
}

// extractParamFieldValue 提取参数字段值
func extractParamFieldValue(block, fieldName string) string {
	// 匹配三引号
	reTriple := regexp.MustCompile(`(?s)\b` + fieldName + `\s*:\s*('''[\s\S]*?'''|"""[\s\S]*?""")`)
	if m := reTriple.FindStringSubmatch(block); len(m) > 1 {
		return cleanGroovyQuotes(m[1])
	}

	// 匹配单/双引号
	reSingle := regexp.MustCompile(`\b` + fieldName + `\s*:\s*('([^'\\]*(?:\\.[^'\\]*)*)'|"([^"\\]*(?:\\.[^"\\]*)*)")`)
	if m := reSingle.FindStringSubmatch(block); len(m) > 1 {
		return cleanGroovyQuotes(m[1])
	}

	// 匹配无引号值
	reRaw := regexp.MustCompile(`\b` + fieldName + `\s*:\s*([^,\)\s]+)`)
	if m := reRaw.FindStringSubmatch(block); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	return ""
}

// extractParamChoices 提取 choices 列表
func extractParamChoices(block string) []string {
	var res []string
	reList := regexp.MustCompile(`(?s)\bchoices\s*:\s*\[([\s\S]*?)\]`)
	if m := reList.FindStringSubmatch(block); len(m) > 1 {
		inner := m[1]
		itemRe := regexp.MustCompile(`('([^'\\]*(?:\\.[^'\\]*)*)'|"([^"\\]*(?:\\.[^"\\]*)*)"|[^,\s]+)`)
		matches := itemRe.FindAllString(inner, -1)
		for _, item := range matches {
			c := cleanGroovyQuotes(strings.TrimSpace(item))
			if c != "" {
				res = append(res, c)
			}
		}
		return res
	}

	// 可能是换行字符串
	reStr := regexp.MustCompile(`\bchoices\s*:\s*('([^'\\]*(?:\\.[^'\\]*)*)'|"([^"\\]*(?:\\.[^"\\]*)*)")`)
	if m := reStr.FindStringSubmatch(block); len(m) > 1 {
		strVal := cleanGroovyQuotes(m[1])
		lines := strings.Split(strVal, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" {
				res = append(res, line)
			}
		}
	}
	return res
}

// parsePipelineParams 从原始脚本中解析 parameters { ... } 中的全部参数定义
func parsePipelineParams(script string) []ParsedJenkinsParam {
	var params []ParsedJenkinsParam
	paramsBlock := extractPipelineBlock(script, "parameters")
	if strings.TrimSpace(paramsBlock) == "" {
		return params
	}

	// 匹配所有参数定义的起始位置（位于行首或缩进后的参数指令，排除属性键如 choiceType 等）
	paramKwRegex := regexp.MustCompile(`(?m)^\s*(reactiveChoice|activeChoice|cascadeChoice|choice|booleanParam|string|text|password)\b`)
	locs := paramKwRegex.FindAllStringIndex(paramsBlock, -1)
	if len(locs) == 0 {
		return params
	}

	for i := 0; i < len(locs); i++ {
		start := locs[i][0]
		end := len(paramsBlock)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		rawChunk := strings.TrimSpace(paramsBlock[start:end])

		kwMatch := paramKwRegex.FindStringSubmatch(rawChunk)
		if len(kwMatch) < 2 {
			continue
		}
		pType := kwMatch[1]

		name := extractParamFieldValue(rawChunk, "name")
		if name == "" {
			continue
		}
		desc := extractParamFieldValue(rawChunk, "description")
		defaultVal := extractParamFieldValue(rawChunk, "defaultValue")
		trimVal := extractParamFieldValue(rawChunk, "trim")

		p := ParsedJenkinsParam{
			Type:         pType,
			Name:         name,
			Description:  desc,
			DefaultValue: defaultVal,
			Trim:         strings.EqualFold(trimVal, "true"),
		}

		switch pType {
		case "choice":
			p.Choices = extractParamChoices(rawChunk)
		case "reactiveChoice", "activeChoice", "cascadeChoice":
			p.ChoiceType = extractParamFieldValue(rawChunk, "choiceType")
			if p.ChoiceType == "" {
				p.ChoiceType = "PT_SINGLE_SELECT"
			}
			p.RandomName = extractParamFieldValue(rawChunk, "randomName")
			p.ReferencedParameters = extractParamFieldValue(rawChunk, "referencedParameters")
			filterableVal := extractParamFieldValue(rawChunk, "filterable")
			p.Filterable = strings.EqualFold(filterableVal, "true")

			// 提取 script: '''...''' 内部的 groovy 脚本
			scriptRe := regexp.MustCompile(`(?s)\bscript\s*:\s*('''([\s\S]*?)'''|"""([\s\S]*?)""")`)
			sMatches := scriptRe.FindAllStringSubmatch(rawChunk, -1)
			if len(sMatches) > 0 {
				bestScript := ""
				for _, sm := range sMatches {
					candidate := cleanGroovyQuotes(sm[1])
					if len(candidate) > len(bestScript) {
						bestScript = candidate
					}
				}
				p.GroovyScript = bestScript
			} else {
				sSingleRe := regexp.MustCompile(`\bscript\s*:\s*('([^'\\]*(?:\\.[^'\\]*)*)'|"([^"\\]*(?:\\.[^"\\]*)*)")`)
				if sm := sSingleRe.FindStringSubmatch(rawChunk); len(sm) > 1 {
					p.GroovyScript = cleanGroovyQuotes(sm[1])
				}
			}

			// sandbox
			sandboxVal := extractParamFieldValue(rawChunk, "sandbox")
			p.Sandbox = strings.EqualFold(sandboxVal, "true")
		}

		params = append(params, p)
	}

	return params
}

// GenerateJobPropertiesXml 根据 rawScript 解析并在新建或更新时生成 Jenkins 标准的 <properties> XML 结构
func GenerateJobPropertiesXml(script string) string {
	params := parsePipelineParams(script)
	hasDisableConcurrent := false

	optionsBlock := extractPipelineBlock(script, "options")
	if strings.Contains(optionsBlock, "disableConcurrentBuilds") {
		hasDisableConcurrent = true
	}

	if len(params) == 0 && !hasDisableConcurrent {
		return "<properties/>"
	}

	var sb strings.Builder
	sb.WriteString("  <properties>\n")

	if hasDisableConcurrent {
		sb.WriteString(`    <org.jenkinsci.plugins.workflow.job.properties.DisableConcurrentBuildsJobProperty>
      <abortPrevious>false</abortPrevious>
    </org.jenkinsci.plugins.workflow.job.properties.DisableConcurrentBuildsJobProperty>` + "\n")
	}

	if len(params) > 0 {
		sb.WriteString("    <hudson.model.ParametersDefinitionProperty>\n")
		sb.WriteString("      <parameterDefinitions>\n")

		for _, p := range params {
			switch p.Type {
			case "booleanParam":
				sb.WriteString(fmt.Sprintf(`        <hudson.model.BooleanParameterDefinition>
          <name>%s</name>
          <description>%s</description>
          <defaultValue>%s</defaultValue>
        </hudson.model.BooleanParameterDefinition>`+"\n", escapeXmlText(p.Name), escapeXmlText(p.Description), p.DefaultValue))

			case "choice":
				var choiceItems strings.Builder
				for _, c := range p.Choices {
					choiceItems.WriteString(fmt.Sprintf("              <string>%s</string>\n", escapeXmlText(c)))
				}
				sb.WriteString(fmt.Sprintf(`        <hudson.model.ChoiceParameterDefinition>
          <name>%s</name>
          <description>%s</description>
          <choices class="java.util.Arrays$ArrayList">
            <a class="string-array">
%s            </a>
          </choices>
        </hudson.model.ChoiceParameterDefinition>`+"\n", escapeXmlText(p.Name), escapeXmlText(p.Description), choiceItems.String()))

			case "string":
				trimStr := "false"
				if p.Trim {
					trimStr = "true"
				}
				sb.WriteString(fmt.Sprintf(`        <hudson.model.StringParameterDefinition>
          <name>%s</name>
          <description>%s</description>
          <defaultValue>%s</defaultValue>
          <trim>%s</trim>
        </hudson.model.StringParameterDefinition>`+"\n", escapeXmlText(p.Name), escapeXmlText(p.Description), escapeXmlText(p.DefaultValue), trimStr))

			case "text":
				trimStr := "false"
				if p.Trim {
					trimStr = "true"
				}
				sb.WriteString(fmt.Sprintf(`        <hudson.model.TextParameterDefinition>
          <name>%s</name>
          <description>%s</description>
          <defaultValue>%s</defaultValue>
          <trim>%s</trim>
        </hudson.model.TextParameterDefinition>`+"\n", escapeXmlText(p.Name), escapeXmlText(p.Description), escapeXmlText(p.DefaultValue), trimStr))

			case "password":
				sb.WriteString(fmt.Sprintf(`        <hudson.model.PasswordParameterDefinition>
          <name>%s</name>
          <description>%s</description>
          <defaultValue>%s</defaultValue>
        </hudson.model.PasswordParameterDefinition>`+"\n", escapeXmlText(p.Name), escapeXmlText(p.Description), escapeXmlText(p.DefaultValue)))

			case "reactiveChoice", "activeChoice", "cascadeChoice":
				sandboxStr := "false"
				if p.Sandbox {
					sandboxStr = "true"
				}
				filterableStr := "false"
				if p.Filterable {
					filterableStr = "true"
				}
				sb.WriteString(fmt.Sprintf(`        <org.biouno.unochoice.ChoiceParameter plugin="uno-choice">
          <name>%s</name>
          <description>%s</description>
          <randomName>%s</randomName>
          <visibleItemCount>1</visibleItemCount>
          <script class="org.biouno.unochoice.model.GroovyScript">
            <secureScript plugin="script-security">
              <script>%s</script>
              <sandbox>%s</sandbox>
            </secureScript>
            <secureFallbackScript plugin="script-security">
              <script></script>
              <sandbox>false</sandbox>
            </secureFallbackScript>
          </script>
          <projectName></projectName>
          <projectFullName></projectFullName>
          <parameters class="linked-hash-map"/>
          <referencedParameters>%s</referencedParameters>
          <choiceType>%s</choiceType>
          <filterable>%s</filterable>
          <filterLength>1</filterLength>
        </org.biouno.unochoice.ChoiceParameter>`+"\n",
					escapeXmlText(p.Name),
					escapeXmlText(p.Description),
					escapeXmlText(p.RandomName),
					escapeXmlText(p.GroovyScript),
					sandboxStr,
					escapeXmlText(p.ReferencedParameters),
					escapeXmlText(p.ChoiceType),
					filterableStr,
				))
			}
		}

		sb.WriteString("      </parameterDefinitions>\n")
		sb.WriteString("    </hudson.model.ParametersDefinitionProperty>\n")
	}

	sb.WriteString("  </properties>")
	return sb.String()
}
