"""
Install:
    pip install -r requirements.txt

Usage:
    python -m uvicorn run-llm-api-server:app --port 8888
"""
import json
from pathlib import Path
from typing import List
from typing import Optional

import requests
import yaml
from fastapi import FastAPI
from google import genai
from google.genai import types
from google.genai.types import GoogleSearch
from pydantic import BaseModel
from starlette.responses import JSONResponse

# Load API keys from config
config_path = Path(__file__).resolve().parent.parent / "configs" / "setting.yaml"
if not config_path.exists():
    raise FileNotFoundError(f"Config file not found at {config_path}")

with open(config_path, "r", encoding="utf-8") as f:
    config = yaml.safe_load(f)

GEMINI_API_KEY = config.get("llm", {}).get("gemini")
OPEN_ROUTER_TOKEN = config.get("llm", {}).get("open_router")

if not GEMINI_API_KEY:
    raise ValueError("GEMINI_API_KEY not found in llm.gemini in configs/setting.yaml")
if not OPEN_ROUTER_TOKEN:
    raise ValueError("OPEN_ROUTER_TOKEN not found in llm.open_router in configs/setting.yaml")

# PROMPT_INJECTION = ' This is a multi-person conversation scenario, and the user input will begin with "{their name}: ". Don\'t start your reply with "{their name}: ". Always reply in Traditional Chinese.'
PROMPT_INJECTION = ''
DEFAULT_SYSTEM_PROMPT = 'You are a helpful assistant.' + PROMPT_INJECTION
OPENROUTER_MODELS = {
    "llama": {
        "model_id": "meta-llama/llama-3.3-70b-instruct:free",
        "max_tokens": 5000,
    },
    "nemotron": {
        "model_id": "nvidia/nemotron-3-super-120b-a12b:free",
        "max_tokens": 5000,
    },
    "glm": {
        "model_id": "z-ai/glm-4.5-air:free",
        "max_tokens": 5000,
    },
}

chat_histories = {
    model: [
        {
            'role': 'system',
            'content': DEFAULT_SYSTEM_PROMPT,
        },
    ]
    for model in OPENROUTER_MODELS
}

genai_client = genai.Client(api_key=GEMINI_API_KEY, http_options={'api_version': 'v1alpha'})
gemini_chat_history: List[dict] = []

app = FastAPI()


@app.exception_handler(Exception)
async def general_exception_handler(_request, ex: Exception):
    error_message = f'Got a unhandled exception: {ex}'
    print(error_message)
    return JSONResponse(
        content={'content': error_message},
    )


class ChatRequest(BaseModel):
    prompt: str
    tools: Optional[str] = None


def chomp(x):
    if x.endswith('\r\n'):
        return x[:-2]
    if x.endswith('\n') or x.endswith('\r'):
        return x[:-1]
    return x


def parse_openrouter_error(resp_json):
    if 'error' in resp_json:
        err = resp_json['error']
        msg = err.get('message', 'Unknown error')
        metadata = err.get('metadata', {})
        # metadata might be a string or a dict
        raw = None
        if isinstance(metadata, dict):
            raw = metadata.get('raw')

        if raw:
            return f"OpenRouter Error: {raw}"
        return f"OpenRouter Error: {msg}"
    return None


@app.post("/api/v1/gemini/chat")
async def get_gemini_response(request: ChatRequest):
    global gemini_chat_history
    gemini_chat_history.append(
        {
            'parts': [{'text': request.prompt}],
            'role': 'user',
        },
    )
    if request.tools and request.tools == 'code_execution':
        tools = [types.Tool(code_execution=types.ToolCodeExecution())]
    elif request.tools and request.tools == 'google_search_tool':
        tools = [types.Tool(google_search=GoogleSearch())]
    else:
        tools = None
    response = genai_client.models.generate_content(
        model='gemini-3-flash-preview',
        config=types.GenerateContentConfig(
            system_instruction=DEFAULT_SYSTEM_PROMPT,
            tools=tools,
            candidate_count=1,
            max_output_tokens=5000,
            temperature=0.5,
            safety_settings=[
                types.SafetySetting(category='HARM_CATEGORY_HATE_SPEECH', threshold='BLOCK_NONE'),
                types.SafetySetting(category='HARM_CATEGORY_DANGEROUS_CONTENT', threshold='BLOCK_NONE'),
                types.SafetySetting(category='HARM_CATEGORY_HARASSMENT', threshold='BLOCK_NONE'),
                types.SafetySetting(category='HARM_CATEGORY_SEXUALLY_EXPLICIT', threshold='BLOCK_NONE'),
                types.SafetySetting(category='HARM_CATEGORY_CIVIC_INTEGRITY', threshold='BLOCK_NONE'),
            ],
        ),
        contents=gemini_chat_history,
    )
    content = ''
    try:
        if not response.candidates[0].content.parts and response.candidates[0].finish_reason:
            return {
                'content': f'Gemini response is empty and finished with reason: {str(response.candidates[0].finish_reason)}',
            }
        for part in response.candidates[0].content.parts:
            if part.thought:
                print(f"Model Thought:\n{part.text}\n")
            elif part.text:
                content += part.text
            elif part.executable_code:
                content += f'Code:\n```\n{part.executable_code.code}\n```'
            elif part.code_execution_result:
                content += f'Code Result:\n```\n{part.code_execution_result.output}\n```'
            else:
                print(f'Got strange response: {response}')
    except Exception as ex:
        print(f'Got exception while parsing Gemini response: {ex}')
        return {'content': f'Got exception while parsing Gemini response: {str(ex)}'}
    gemini_chat_history.append(
        {
            'parts': [{'text': content}],
            'role': 'model',
        },
    )
    return {'content': content}


@app.get("/api/v1/gemini/reset")
async def reset_gemini_session():
    global gemini_chat_history
    gemini_chat_history = []
    return {"content": "Session reset"}


@app.post("/api/v1/{model_name}/chat")
async def get_openrouter_response(model_name: str, request: ChatRequest):
    if model_name not in OPENROUTER_MODELS:
        return JSONResponse(
            status_code=404,
            content={'content': f'Model {model_name} not supported!'},
        )

    model_cfg = OPENROUTER_MODELS[model_name]
    history = chat_histories[model_name]

    history.append(
        {
            'role': 'user',
            'content': f'{request.prompt}',
        },
    )

    payload = {
        'model': model_cfg['model_id'],
        'messages': history,
        'max_tokens': model_cfg.get('max_tokens', 5000),
    }
    if model_cfg.get('include_reasoning'):
        payload['include_reasoning'] = True

    response = requests.post(
        url='https://openrouter.ai/api/v1/chat/completions',
        headers={
            'Authorization': f'Bearer {OPEN_ROUTER_TOKEN}',
        },
        data=json.dumps(payload),
    )

    resp_json = response.json()
    if model_name == 'deepseek':
        print(resp_json)

    error_msg = parse_openrouter_error(resp_json)
    if error_msg:
        return {'content': error_msg}

    if 'choices' not in resp_json or not resp_json['choices']:
        return {'content': f'OpenRouter returned unexpected response: {resp_json}'}

    msg_content = resp_json['choices'][0]['message'].get('content')
    if not msg_content:
        return {'content': 'No response.'}

    content = chomp(msg_content)
    history.append(
        {
            'role': 'assistant',
            'content': f'{content}',
        },
    )
    return {'content': content}


@app.get("/api/v1/{model_name}/reset")
async def reset_openrouter_session(model_name: str):
    if model_name not in OPENROUTER_MODELS:
        return JSONResponse(
            status_code=404,
            content={'content': f'Model {model_name} not supported!'},
        )

    chat_histories[model_name] = [
        {
            'role': 'system',
            'content': DEFAULT_SYSTEM_PROMPT,
        },
    ]
    return {"message": f"Session for {model_name} reset"}


@app.patch("/api/v1/system_prompt")
async def update_system_prompt(request: ChatRequest):
    global DEFAULT_SYSTEM_PROMPT, PROMPT_INJECTION
    global gemini_chat_history, chat_histories

    system_prompt = request.prompt
    if system_prompt is None or system_prompt.lower() in ('default', 'none', ''):
        DEFAULT_SYSTEM_PROMPT = 'You are a confident, efficient, and helpful assistant.' + PROMPT_INJECTION
    else:
        DEFAULT_SYSTEM_PROMPT = system_prompt + PROMPT_INJECTION

    # Reset and update all histories
    gemini_chat_history = []
    for model in chat_histories:
        chat_histories[model] = [
            {
                'role': 'system',
                'content': DEFAULT_SYSTEM_PROMPT,
            },
        ]

    return {'content': f'System prompt updated to `{DEFAULT_SYSTEM_PROMPT}` and all sessions reset.'}
