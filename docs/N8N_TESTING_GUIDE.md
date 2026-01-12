# N8N Workflow Testing Guide

## 📊 Current Status

✅ **N8N is running** on `http://localhost:5775`
✅ **PostgreSQL** is running for n8n database
✅ **All workflows are loaded** but **NOT ACTIVE**

## 🚨 Issue Found

Your workflows are loaded in n8n but they need to be **activated** to receive webhook requests:
- ❌ 01_Auth_Helper - Not Active
- ❌ 02_WhatsApp_Sender - Not Active  
- ❌ 03_High_Traffic_Alert - Not Active

## ⚙️ How to Activate Workflows

### Step 1: Access N8N Dashboard
Open your browser and go to: **http://localhost:5775**

### Step 2: Activate Each Workflow
1. Click on **"03_High_Traffic_Alert"** to open it
2. Look for the **toggle switch** in the top-right corner of the editor
3. **Turn ON** the toggle to activate the workflow
4. Repeat for the other workflows:
   - 01_Auth_Helper
   - 02_WhatsApp_Sender

### Step 3: Test the High Traffic Alert

Once activated, run this command to test:

```bash
curl -s -X POST http://localhost:5775/webhook/high-traffic-alert-webhook \
  -H "Content-Type: application/json" \
  -d '{
    "status": "firing",
    "value": 95,
    "timestamp": "'$(date -u +'%Y-%m-%dT%H:%M:%SZ')'",
    "service": "api-service",
    "description": "CPU usage exceeded threshold"
  }'
```

Or use the convenience script:
```bash
bash /tmp/test_n8n_workflow.sh
```

## 📱 Expected Behavior

When workflow is activated and triggered:

1. **Alert Webhook** receives POST request with `status: "firing"` and `value > 80`
2. **If condition** checks if high traffic (value > 80%)
3. **Prepare Data** node formats the message with emoji and alert details
4. **Send WhatsApp Alert** calls the WhatsApp sender workflow
5. **WhatsApp Message** is sent to **6285173207755** (configured in workflow)

Expected message on WhatsApp:
```
🚨 *High Traffic Alert!* Server CPU is at 95%!
```

## 🔗 Workflow Dependencies

Your workflows are connected:
```
01_Auth_Helper (1)
   ↓
02_WhatsApp_Sender (2)
   ↑
03_High_Traffic_Alert (3) ← Main webhook entry point
```

**Flow:**
1. High Traffic Alert receives alert → 
2. Checks if value > 80 → 
3. Calls Prepare Data → 
4. Sends to WhatsApp via workflow execution → 
5. WhatsApp Sender uses Auth Helper to get session → 
6. Sends message to recipient

## ✅ Integration Points

- **API Port**: 6221 (your Go API service)
- **WhatsApp gRPC**: 50042 (for sending WhatsApp messages)
- **N8N Port**: 5775 (workflow automation)
- **WhatsApp Recipient**: 6285173207755

## 🐛 Troubleshooting

If message doesn't arrive:
1. Check n8n executions log (click the hamburger menu → Executions)
2. Verify WhatsApp gRPC service is running: `podman ps | grep whatsapp`
3. Check if session is properly authenticated in Auth Helper workflow
4. Review the WhatsApp Sender workflow for any errors

## 📝 Test Command

After activating workflows, use this to test:

```bash
# Simple test
curl -s -X POST http://localhost:5775/webhook-test/alert \
  -H "Content-Type: application/json" \
  -d '{"status": "firing", "value": 95, "phone": "6285173207755", "message": "🚨 High Traffic Alert!"}'

# Full test with all fields
/tmp/test_n8n_workflow.sh
```

## 🎯 Next Steps

1. ✅ Go to http://localhost:5775
2. ✅ Activate all 3 workflows
3. ✅ Run test command
4. ✅ Check phone for WhatsApp message
5. ✅ Review n8n execution logs if needed

---

**Created**: 2026-01-09
**Service Platform**: v1.0.0
