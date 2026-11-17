// Imprime o JSON de um retexture: node retex.mjs <classe> <rare|epic|legend>
import { SOURCES, STYLE, TIERS } from './prompts.mjs';
const [cls, tier] = process.argv.slice(2);
process.stdout.write(JSON.stringify({ input_task_id: SOURCES[cls], text_style_prompt: `${TIERS[tier][cls]}, ${STYLE}`, enable_original_uv: true, enable_pbr: false, ai_model: 'meshy-6' }));
