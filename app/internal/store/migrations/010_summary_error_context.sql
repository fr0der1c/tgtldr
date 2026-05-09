alter table summaries
add column if not exists error_context text not null default '';

alter table summaries
add column if not exists error_system_prompt text not null default '';

alter table summaries
add column if not exists error_user_prompt text not null default '';

create or replace function tgtldr_try_parse_jsonb(value text)
returns jsonb
language plpgsql
as $$
begin
  return value::jsonb;
exception when others then
  return null;
end;
$$;

with parsed as (
  select id, tgtldr_try_parse_jsonb(error_context) as context_json
  from summaries
  where error_context <> ''
)
update summaries as s
set error_system_prompt = coalesce(nullif(s.error_system_prompt, ''), parsed.context_json->>'systemPrompt', ''),
    error_user_prompt = coalesce(nullif(s.error_user_prompt, ''), parsed.context_json->>'userPrompt', ''),
    error_context = jsonb_pretty(parsed.context_json - 'systemPrompt' - 'userPrompt')
from parsed
where s.id = parsed.id
  and parsed.context_json is not null
  and (parsed.context_json ? 'systemPrompt' or parsed.context_json ? 'userPrompt');

drop function tgtldr_try_parse_jsonb(text);
