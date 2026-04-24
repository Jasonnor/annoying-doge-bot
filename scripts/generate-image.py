"""
Run command:
python generate-image.py -p "shiba"
"""

import argparse
import os
import uuid
from pathlib import Path

import boto3
import yaml
from huggingface_hub import InferenceClient

# Load API keys from config
config_path = Path(__file__).resolve().parent.parent / "configs" / "setting.yaml"
if not config_path.exists():
    raise FileNotFoundError(f"Config file not found at {config_path}")

with open(config_path, "r", encoding="utf-8") as f:
    config = yaml.safe_load(f)

HUGGING_FACE_TOKEN = config.get("llm", {}).get("hugging_face")
S3_ACCESS_KEY_ID = config.get("s3", {}).get("access_key_id")
S3_SECRET_ACCESS_KEY = config.get("s3", {}).get("secret_access_key")
S3_BUCKET_NAME = config.get("s3", {}).get("bucket_name")


def generate_image(prompt):
    client = InferenceClient(
        provider="hf-inference",
        api_key=HUGGING_FACE_TOKEN,
    )

    image = client.text_to_image(
        prompt,
        model="black-forest-labs/FLUX.1-schnell",
    )

    # Create images directory if it doesn't exist
    script_root = Path(__file__).resolve().parent.parent
    output_dir = f'{script_root}/images'
    os.makedirs(output_dir, exist_ok=True)

    # Save image with unique filename
    image_path = f'{output_dir}/{uuid.uuid4()}.jpg'
    image.save(image_path)

    # Upload to S3
    key_name = f'jason-wu/{uuid.uuid4()}.jpg'
    session = boto3.Session(
        aws_access_key_id=S3_ACCESS_KEY_ID,
        aws_secret_access_key=S3_SECRET_ACCESS_KEY,
    )
    s3 = session.resource('s3')
    s3.Bucket(S3_BUCKET_NAME).upload_file(image_path, key_name)
    print(f'https://{S3_BUCKET_NAME}.s3.us-east-1.amazonaws.com/{key_name}')

    return image_path


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument(
        '--prompt',
        '-p',
        required=True,
        type=str,
        help='Prompt to generate an image',
    )
    generate_image(parser.parse_args().prompt)
