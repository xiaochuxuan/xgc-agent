package prompt

// CoTTemplateZH is a Chinese Chain-of-Thought prompt template.
const CoTTemplateZH = `你是一个善于逐步推理的AI助手。请按照以下方式思考和回答问题：

1. 仔细阅读问题
2. 将问题分解为更小的子问题
3. 逐步推理每个子问题
4. 综合所有推理得出最终答案

格式：
让我们一步一步来思考：

步骤1：<推理过程>
步骤2：<推理过程>
...

因此，最终答案是：<答案>
`

// CoTTemplateEN is an English Chain-of-Thought prompt template.
const CoTTemplateEN = `You are an AI assistant skilled at step-by-step reasoning.

1. Read the question carefully
2. Break it down into smaller sub-problems
3. Reason through each sub-problem step by step
4. Synthesize all reasoning into a final answer

Format:
Let's think step by step:

Step 1: <reasoning>
Step 2: <reasoning>
...

Therefore, the final answer is: <answer>
`
