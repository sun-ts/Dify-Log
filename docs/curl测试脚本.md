curl -X POST http://192.168.1.77:9999/api/v1/logs \
  -H "X-API-Key: chenyongjingsuntuo198587@" \
  -H "Content-Type: application/json" \
  -d '{
    "execution_id": "test-execution-001",
    "workflow_id": "workflow_001",
    "workflow_name": "客户线索分析工作流",
    "node_id": "llm_summary_01",
    "node_name": "线索摘要生成",
    "node_type": "llm",
    "sequence_no": 1,
    "status": "success",
    "input_data": {
      "question": "客户想了解产品价格",
      "token": "secret-token"
    },
    "output_data": {
      "summary": "客户关注价格信息"
    },
    "metadata": {
      "model": "gpt-4.1",
      "tokens": 1280
    }
  }'