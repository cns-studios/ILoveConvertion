import io
import logging
import time

from flask import Flask, request, jsonify, send_file
from PIL import Image
from rembg import remove, new_session

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(message)s",
)
logger = logging.getLogger("rembg-service")

app = Flask(__name__)

logger.info("Loading silueta model...")
start = time.time()
SESSION = new_session("silueta")
logger.info(f"Model loaded in {time.time() - start:.1f}s")

MODEL_READY = True


@app.route("/health", methods=["GET"])
def health():
    """Docker health check + API readiness gate."""
    if not MODEL_READY:
        return jsonify({"status": "loading", "model": "silueta"}), 503
    return jsonify({"status": "ok", "model": "silueta"}), 200


@app.route("/remove-bg", methods=["POST"])
def remove_background():
    """
    Accepts: multipart/form-data with field 'file'
    Optional query param: ?format=png|webp (default: png)
    Returns: processed image bytes with Content-Type
    """
    if "file" not in request.files:
        return jsonify({"error": "No file provided. Use field name 'file'."}), 400

    file = request.files["file"]
    if file.filename == "":
        return jsonify({"error": "Empty filename."}), 400

    output_format = request.args.get("format", "png").lower()
    if output_format not in ("png", "webp"):
        return jsonify({"error": f"Unsupported output format: {output_format}. Use png or webp."}), 400

    try:
        start = time.time()
        input_bytes = file.read()
        input_size = len(input_bytes)
        logger.info(f"Processing {file.filename} ({input_size / 1024:.1f} KB), output={output_format}")

        result_bytes = remove(
            input_bytes,
            session=SESSION,
            alpha_matting=False,
            post_process_mask=True,
        )

        img = Image.open(io.BytesIO(result_bytes)).convert("RGBA")

        output_buffer = io.BytesIO()
        if output_format == "webp":
            img.save(output_buffer, format="WEBP", quality=90, lossless=False)
            mime = "image/webp"
        else:
            img.save(output_buffer, format="PNG", optimize=True)
            mime = "image/png"

        output_buffer.seek(0)
        output_size = output_buffer.getbuffer().nbytes
        elapsed = time.time() - start

        logger.info(
            f"Done: {file.filename} → {output_format} "
            f"({output_size / 1024:.1f} KB, {elapsed:.1f}s)"
        )

        return send_file(
            output_buffer,
            mimetype=mime,
            as_attachment=False,
            download_name=f"result.{output_format}",
        )

    except Exception as e:
        logger.error(f"Processing failed for {file.filename}: {e}", exc_info=True)
        return jsonify({"error": f"Processing failed: {str(e)}"}), 500


@app.errorhandler(413)
def too_large(e):
    return jsonify({"error": "File too large."}), 413


@app.errorhandler(500)
def server_error(e):
    return jsonify({"error": "Internal server error."}), 500


if __name__ == "__main__":
    app.run(host="0.0.0.0", port=5000, debug=False)