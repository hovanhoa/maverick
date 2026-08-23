import * as React from 'react';
import { useLocation } from 'react-router-dom';
import { PaperAirplaneIcon, EyeIcon, EyeSlashIcon, ArrowPathIcon } from '@heroicons/react/24/outline';
import { Card } from '../components/ui/Card';
import { Button } from '../components/ui/Button';
import { ErrorAlert } from '../components/ui/Alert';
import { Field, TextInput, Textarea, Select } from '../components/ui/Field';
import { testChatCompletion } from '../lib/api';
import type { ChatMessage, ChatUsage } from '../lib/api';

// Mirrors the known models in internal/usage/pricing.go - the gateway itself
// passes any "provider/model" straight through, so this is only a curated
// shortlist for convenience. "Custom..." falls back to free text for
// anything else (a new model, or a provider not listed here).
const MODEL_GROUPS: { provider: string; models: string[] }[] = [
  {
    provider: 'anthropic',
    models: ['claude-opus-5', 'claude-sonnet-5', 'claude-haiku-4-5-20251001']
  },
  {
    provider: 'openai',
    models: ['gpt-5', 'gpt-5-mini', 'gpt-4o', 'gpt-4o-mini']
  },
  {
    provider: 'gemini',
    models: ['gemini-2.5-pro', 'gemini-2.5-flash', 'gemini-2.5-flash-lite']
  }
];

const CUSTOM_MODEL = '__custom__';

interface Turn {
  id: number;
  message: ChatMessage;
  usage?: ChatUsage;
  latencyMs?: number;
  error?: string;
}

let nextTurnId = 1;

export function PlaygroundPage() {
  const location = useLocation();
  const prefillKey = (location.state as { apiKey?: string } | null)?.apiKey;

  const [key, setKey] = React.useState(prefillKey ?? '');
  const [showKey, setShowKey] = React.useState(false);
  const [model, setModel] = React.useState('');
  const [useCustomModel, setUseCustomModel] = React.useState(false);
  const [systemPrompt, setSystemPrompt] = React.useState('');

  const [turns, setTurns] = React.useState<Turn[]>([]);
  const [draft, setDraft] = React.useState('');
  const [sending, setSending] = React.useState(false);

  const scrollRef = React.useRef<HTMLDivElement>(null);

  React.useEffect(() => {
    scrollRef.current?.scrollTo({ top: scrollRef.current.scrollHeight, behavior: 'smooth' });
  }, [turns, sending]);

  const canSend = key.trim() !== '' && model.trim() !== '' && draft.trim() !== '' && !sending;

  const handleReset = () => {
    setTurns([]);
    setDraft('');
  };

  const handleSend = async () => {
    if (!canSend) return;

    const userTurn: Turn = { id: nextTurnId++, message: { role: 'user', content: draft.trim() } };
    const history = [...turns, userTurn];
    setTurns(history);
    setDraft('');
    setSending(true);

    const messages: ChatMessage[] = systemPrompt.trim()
      ? [{ role: 'system', content: systemPrompt.trim() }, ...history.map((t) => t.message)]
      : history.map((t) => t.message);

    const start = performance.now();
    try {
      const resp = await testChatCompletion(key.trim(), { model: model.trim(), messages });
      const latencyMs = performance.now() - start;
      setTurns((prev) => [
        ...prev,
        { id: nextTurnId++, message: resp.choices[0].message, usage: resp.usage, latencyMs }
      ]);
    } catch (err) {
      const latencyMs = performance.now() - start;
      setTurns((prev) => [
        ...prev,
        {
          id: nextTurnId++,
          message: { role: 'assistant', content: '' },
          error: err instanceof Error ? err.message : String(err),
          latencyMs
        }
      ]);
    } finally {
      setSending(false);
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  };

  return (
    <div className="flex h-[calc(100vh-6rem)] flex-col space-y-4">
      <div className="flex items-start justify-between gap-4">
        <div>
          <h1 className="text-xl font-semibold text-neutral-900 dark:text-white">Playground</h1>
          <p className="mt-1 text-sm text-neutral-500 dark:text-neutral-400">
            Test an API key against the live gateway - chat with it to check the key works, is on the team's
            allowlist, and passes policy.
          </p>
        </div>
        <Button onClick={handleReset} disabled={turns.length === 0 && !draft}>
          <ArrowPathIcon className="h-4 w-4" />
          Reset
        </Button>
      </div>

      <Card className="grid shrink-0 grid-cols-1 gap-4 p-4 sm:grid-cols-3">
        <Field label="API key">
          <div className="flex gap-2">
            <TextInput
              type={showKey ? 'text' : 'password'}
              value={key}
              onChange={(e) => setKey(e.target.value)}
              placeholder="sk_..."
              autoComplete="off"
              spellCheck={false}
            />
            <Button type="button" onClick={() => setShowKey((v) => !v)} title={showKey ? 'Hide key' : 'Show key'}>
              {showKey ? <EyeSlashIcon className="h-4 w-4" /> : <EyeIcon className="h-4 w-4" />}
            </Button>
          </div>
        </Field>
        <Field label="Model">
          {useCustomModel ? (
            <div className="flex gap-2">
              <TextInput
                value={model}
                onChange={(e) => setModel(e.target.value)}
                placeholder="anthropic/claude-sonnet-5"
                autoFocus
              />
              <Button
                type="button"
                onClick={() => {
                  setUseCustomModel(false);
                  setModel('');
                }}
                title="Pick from list instead"
              >
                List
              </Button>
            </div>
          ) : (
            <Select
              value={model}
              onChange={(e) => {
                if (e.target.value === CUSTOM_MODEL) {
                  setUseCustomModel(true);
                  setModel('');
                } else {
                  setModel(e.target.value);
                }
              }}
            >
              <option value="" disabled>
                Select a model…
              </option>
              {MODEL_GROUPS.map((group) => (
                <optgroup key={group.provider} label={group.provider}>
                  {group.models.map((m) => (
                    <option key={m} value={`${group.provider}/${m}`}>
                      {m}
                    </option>
                  ))}
                </optgroup>
              ))}
              <option value={CUSTOM_MODEL}>Custom…</option>
            </Select>
          )}
        </Field>
        <Field label="System prompt (optional)">
          <TextInput value={systemPrompt} onChange={(e) => setSystemPrompt(e.target.value)} placeholder="You are a helpful assistant" />
        </Field>
      </Card>

      <Card className="flex min-h-0 flex-1 flex-col">
        <div ref={scrollRef} className="flex-1 space-y-4 overflow-y-auto p-5">
          {turns.length === 0 && (
            <div className="flex h-full items-center justify-center text-sm text-neutral-400">
              Send a message to start the conversation.
            </div>
          )}
          {turns.map((turn) => (
            <div key={turn.id} className={turn.message.role === 'user' ? 'flex justify-end' : 'flex justify-start'}>
              <div className="max-w-[80%] space-y-1">
                {turn.error ? (
                  <ErrorAlert message={turn.error} />
                ) : (
                  <div
                    className={
                      turn.message.role === 'user'
                        ? 'whitespace-pre-wrap rounded-2xl rounded-tr-sm bg-primary-600 px-4 py-2.5 text-sm text-white'
                        : 'whitespace-pre-wrap rounded-2xl rounded-tl-sm bg-neutral-100 px-4 py-2.5 text-sm text-neutral-900 dark:bg-white/5 dark:text-white'
                    }
                  >
                    {turn.message.content}
                  </div>
                )}
                {(turn.usage || turn.latencyMs !== undefined) && (
                  <p
                    className={
                      turn.message.role === 'user'
                        ? 'text-right text-xs text-neutral-400'
                        : 'text-left text-xs text-neutral-400'
                    }
                  >
                    {turn.usage && `${turn.usage.total_tokens} tokens · `}
                    {turn.latencyMs !== undefined && `${Math.round(turn.latencyMs)} ms`}
                  </p>
                )}
              </div>
            </div>
          ))}
          {sending && (
            <div className="flex justify-start">
              <div className="rounded-2xl rounded-tl-sm bg-neutral-100 px-4 py-2.5 text-sm text-neutral-400 dark:bg-white/5">
                Thinking…
              </div>
            </div>
          )}
        </div>

        <div className="flex items-end gap-2 border-t border-neutral-950/5 p-4 dark:border-white/10">
          <Textarea
            rows={2}
            className="resize-none"
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="Type a message… (Enter to send, Shift+Enter for a new line)"
          />
          <Button variant="primary" onClick={handleSend} disabled={!canSend}>
            <PaperAirplaneIcon className="h-4 w-4" />
            Send
          </Button>
        </div>
      </Card>
    </div>
  );
}
