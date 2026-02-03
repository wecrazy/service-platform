from transformers import AutoTokenizer, AutoModelForCausalLM, TrainingArguments, Trainer, DataCollatorForLanguageModeling
from datasets import load_dataset
from peft import get_peft_model, LoraConfig, TaskType
import torch
import os
import sys

# $env:HF_TOKEN="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"

# Load token from environment
# token = os.getenv("HF_TOKEN")
token = "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"
if not token:
    print("❌ HF_TOKEN not set. Please set it as an environment variable.")
    sys.exit(1)

# Model ID
model_id = "google/gemma-2-2b"

# Load tokenizer and base model with 8-bit loading
tokenizer = AutoTokenizer.from_pretrained(model_id, token=token)
model = AutoModelForCausalLM.from_pretrained(
    model_id,
    token=token,
    device_map="auto",
    load_in_8bit=True
)

# Apply LoRA configuration
peft_config = LoraConfig(
    r=8,
    lora_alpha=16,
    target_modules=["q_proj", "v_proj"],
    lora_dropout=0.1,
    bias="none",
    task_type=TaskType.CAUSAL_LM
)
model = get_peft_model(model, peft_config)

# Load dataset
dataset = load_dataset("json", data_files="data_train-chatbot.jsonl", split="train")

# Tokenization
def tokenize(example):
    prompt = f"<bos>Instruction: {example['instruction']}\nOutput: {example['output']}</s>"
    return tokenizer(prompt, truncation=True, padding="max_length", max_length=512)

dataset = dataset.map(tokenize)

# Training arguments
args = TrainingArguments(
    per_device_train_batch_size=2,
    gradient_accumulation_steps=4,
    num_train_epochs=3,
    learning_rate=2e-4,
    fp16=True,
    logging_steps=10,
    output_dir="./gemma2_2b-lora-out",
    save_total_limit=2,
    save_strategy="epoch",
    report_to="none",  # avoid wandb unless explicitly wanted
)

# Trainer setup
trainer = Trainer(
    model=model,
    train_dataset=dataset,
    args=args,
    data_collator=DataCollatorForLanguageModeling(tokenizer, mlm=False),
)

# Start training
trainer.train()

# Save LoRA fine-tuned model and tokenizer
model.save_pretrained("gemma2_2b-lora-finetuned")
tokenizer.save_pretrained("gemma2_2b-lora-finetuned")
