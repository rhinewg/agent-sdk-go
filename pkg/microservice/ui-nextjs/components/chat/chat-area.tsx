'use client';

import React, { useState, useRef, useEffect } from 'react';
import { AgentConfig, ChatMessage, ContentPart } from '@/types/agent';
import { agentAPI } from '@/lib/api';
import { Card } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Textarea } from '@/components/ui/textarea';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Badge } from '@/components/ui/badge';
import { Switch } from '@/components/ui/switch';
import { Send, Trash2, Loader2, RefreshCw, Building2 } from 'lucide-react';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { ChatMessage as ChatMessageComponent } from './chat-message';
import { FileTransfer, UploadedFileInfo } from './file-transfer';

interface ChatAreaProps {
  agentConfig: AgentConfig | null;
}

type ThinkingStepItem = {
  content: string;
  timestamp: number;
};

type ToolCallItem = {
  id?: string;
  name: string;
  status: string;
  arguments?: string;
  result?: string;
  timestamp: number;
};

export function ChatArea({ agentConfig }: ChatAreaProps) {
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [input, setInput] = useState('');
  const [isLoading, setIsLoading] = useState(false);
  const [pendingImages, setPendingImages] = useState<Array<{ name: string; url: string; size: number }>>([]);
  const [conversationId, setConversationId] = useState<string>('');
  const [organizationId, setOrganizationId] = useState<string>('');
  const [charCount, setCharCount] = useState(0);
  const [streamingEnabled, setStreamingEnabled] = useState(true);
  const [uploadEnabled, setUploadEnabled] = useState(false);
  const [uploadedFile, setUploadedFile] = useState<UploadedFileInfo | null>(null);
  const [thinkingSteps, setThinkingSteps] = useState<ThinkingStepItem[]>([]);
  const [toolCalls, setToolCalls] = useState<ToolCallItem[]>([]);
  const [showThinking, setShowThinking] = useState(false);
  const [showToolCalls, setShowToolCalls] = useState(false);

  const messagesEndRef = useRef<HTMLDivElement>(null);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const imageInputRef = useRef<HTMLInputElement>(null);

  const stringifyMaybe = (val: unknown) => {
    if (val === undefined || val === null) return undefined;
    if (typeof val === 'string') return val;
    try {
      return JSON.stringify(val, null, 2);
    } catch {
      return String(val);
    }
  };

  useEffect(() => {
    scrollToBottom();
  }, [messages]);

  useEffect(() => {
    setCharCount(input.length);
    adjustTextareaHeight();
  }, [input]);

  const scrollToBottom = () => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  };

  const adjustTextareaHeight = () => {
    const textarea = textareaRef.current;
    if (textarea) {
      textarea.style.height = 'auto';
      textarea.style.height = `${Math.min(textarea.scrollHeight, 120)}px`;
    }
  };

  const generateConversationId = () => {
    return `conv_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`;
  };

  const generateNewConversationId = () => {
    // Use built-in crypto.randomUUID() for proper UUID v4 generation
    const newId = crypto.randomUUID();
    setConversationId(newId);
    // Clear messages when generating new conversation
    setMessages([]);
  };

  const addMessage = (message: ChatMessage) => {
    setMessages(prev => [...prev, message]);
  };

  const readFileAsDataURL = (file: File): Promise<string> => {
    return new Promise((resolve, reject) => {
      const reader = new FileReader();
      reader.onload = () => resolve(String(reader.result || ''));
      reader.onerror = () => reject(new Error('Failed to read file'));
      reader.readAsDataURL(file);
    });
  };

  const handleImageFiles = async (files: FileList | null) => {
    if (!files || files.length === 0) return;

    const maxImageBytes = 20 * 1024 * 1024; // keep aligned with server default (20MB)
    const newItems: Array<{ name: string; url: string; size: number }> = [];

    for (const file of Array.from(files)) {
      if (!file.type.startsWith('image/')) {
        addMessage({
          role: 'assistant',
          content: `Error: Unsupported file type: ${file.type || file.name}`,
          timestamp: Date.now(),
          id: `msg_${Date.now()}_error`,
        });
        continue;
      }
      if (file.size > maxImageBytes) {
        addMessage({
          role: 'assistant',
          content: `Error: Image too large: ${file.name} (${file.size} bytes, max ${maxImageBytes})`,
          timestamp: Date.now(),
          id: `msg_${Date.now()}_error`,
        });
        continue;
      }

      try {
        const url = await readFileAsDataURL(file);
        if (!url.startsWith('data:image/')) {
          addMessage({
            role: 'assistant',
            content: `Error: Invalid image data URL for: ${file.name}`,
            timestamp: Date.now(),
            id: `msg_${Date.now()}_error`,
          });
          continue;
        }
        newItems.push({ name: file.name, url, size: file.size });
      } catch (err) {
        addMessage({
          role: 'assistant',
          content: `Error: Failed to read image: ${file.name}`,
          timestamp: Date.now(),
          id: `msg_${Date.now()}_error`,
        });
      }
    }

    if (newItems.length > 0) {
      setPendingImages(prev => [...prev, ...newItems]);
    }
  };

  const updateLastMessage = (content: string) => {
    setMessages(prev => {
      const newMessages = [...prev];
      if (newMessages.length > 0 && newMessages[newMessages.length - 1].role === 'assistant') {
        newMessages[newMessages.length - 1] = {
          ...newMessages[newMessages.length - 1],
          content: content,
        };
      }
      return newMessages;
    });
  };

  const sendMessage = async () => {
    const inputTrimmed = input.trim();
    const hasUploadedFile = Boolean(uploadEnabled && uploadedFile);
    if ((!inputTrimmed && pendingImages.length === 0 && !hasUploadedFile) || isLoading) return;

    const baseVisibleContent =
      inputTrimmed || (pendingImages.length > 0 ? `[sent ${pendingImages.length} image(s)]` : '');
    const userVisibleContent = hasUploadedFile
      ? (baseVisibleContent
          ? `${baseVisibleContent}\n\n[file]: ${uploadedFile!.name}`
          : `[sent file: ${uploadedFile!.name}]`)
      : baseVisibleContent;

    // 清空上一轮的思考步骤和工具调用展示
    setThinkingSteps([]);
    setToolCalls([]);

    // 拼接上传文件信息到内容，作为参数传递（使用服务器绝对路径）
    const payloadContent = (() => {
      if (!hasUploadedFile) return inputTrimmed;
      const fileMeta = `[uploaded_file]: name=${uploadedFile!.name}, path=${uploadedFile!.absolutePath}`;
      return inputTrimmed ? `${inputTrimmed}\n\n${fileMeta}` : fileMeta;
    })();

    const userMessage: ChatMessage = {
      role: 'user',
      // Preserve the real payload (including uploaded file metadata) in `content`
      // while showing a friendlier version in the UI.
      content: payloadContent,
      display_content: userVisibleContent,
      timestamp: Date.now(),
      id: `msg_${Date.now()}_user`,
    };

    addMessage(userMessage);
    setInput('');
    setPendingImages([]);
    setIsLoading(true);

    // Generate conversation ID if not exists
    const currentConversationId = conversationId || generateConversationId();
    if (!conversationId) {
      setConversationId(currentConversationId);
    }

    try {
      const contentParts: ContentPart[] = [];
      if (payloadContent) {
        contentParts.push({ type: 'text', text: payloadContent });
      }
      for (const img of pendingImages) {
        contentParts.push({
          type: 'image_url',
          image_url: { url: img.url, detail: 'auto' },
        });
      }

      if (streamingEnabled) {
        // Streaming response
        const assistantMessage: ChatMessage = {
          role: 'assistant',
          content: '',
          timestamp: Date.now(),
          id: `msg_${Date.now()}_assistant`,
        };
        addMessage(assistantMessage);

        const stream = agentAPI.streamAgent({
          input: payloadContent,
          content_parts: contentParts.length > 0 ? contentParts : undefined,
          conversation_id: currentConversationId,
          org_id: organizationId || undefined,
        });

        let fullContent = '';
        for await (const eventData of stream) {
          if (eventData.error) {
            throw new Error(eventData.error);
          }

          if (eventData.type === 'content' && eventData.content) {
            fullContent += eventData.content;
            updateLastMessage(fullContent);
          }

          // Thinking steps
          if (eventData.thinking_step) {
            setThinkingSteps((prev) => [
              ...prev,
              { content: eventData.thinking_step!, timestamp: Date.now() },
            ]);
          }

          // MCP/Tool calls
          if (eventData.tool_call) {
            setToolCalls((prev) => {
              const existingIndex = prev.findIndex(
                (item) => {
                  const incomingId = eventData.tool_call?.id || eventData.tool_call?.name;
                  return incomingId && (item.id === incomingId || item.name === eventData.tool_call?.name);
                },
              );
              const updatedItem: ToolCallItem = {
                id: eventData.tool_call?.id,
                name: eventData.tool_call?.name ?? 'unknown_tool',
                status: eventData.tool_call?.status ?? 'unknown',
                arguments: stringifyMaybe(eventData.tool_call?.arguments),
                result: stringifyMaybe(eventData.tool_call?.result),
                timestamp: Date.now(),
              };

              if (existingIndex >= 0) {
                const next = [...prev];
                next[existingIndex] = { ...next[existingIndex], ...updatedItem };
                return next;
              }
              return [...prev, updatedItem];
            });
          }

          if (eventData.is_final || eventData.type === 'done') {
            break;
          }
        }
      } else {
        // Non-streaming response
        const response = await agentAPI.runAgent({
          input: payloadContent,
          content_parts: contentParts.length > 0 ? contentParts : undefined,
          conversation_id: currentConversationId,
          org_id: organizationId || undefined,
        });

        const assistantMessage: ChatMessage = {
          role: 'assistant',
          content: response.output,
          timestamp: Date.now(),
          id: `msg_${Date.now()}_assistant`,
        };
        addMessage(assistantMessage);
      }
    } catch (error) {
      console.error('Error sending message:', error);
      const errorMessage: ChatMessage = {
        role: 'assistant',
        content: `Error: ${error instanceof Error ? error.message : 'Unknown error occurred'}`,
        timestamp: Date.now(),
        id: `msg_${Date.now()}_error`,
      };
      addMessage(errorMessage);
    } finally {
      setIsLoading(false);
    }
  };

  const clearChat = () => {
    setMessages([]);
    setConversationId('');
    setThinkingSteps([]);
    setToolCalls([]);
    // Don't clear organization ID as user might want to keep it
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      sendMessage();
    }
  };

  return (
    <div className="flex flex-col h-full">
      {/* Messages Area */}
      <div className="flex-1 overflow-hidden">
        <ScrollArea className="h-full">
          <div className="p-4">
            {messages.length === 0 ? (
              <div className="flex items-center justify-center min-h-[400px]">
                <Card className="p-8 text-center max-w-md">
                  <h2 className="text-2xl font-semibold mb-4">Welcome to Agent UI</h2>
                  <p className="text-muted-foreground mb-4">
                    Start a conversation by typing a message below.
                  </p>
                  {agentConfig && (
                    <div className="space-y-2">
                      <Badge variant="outline">Agent: {agentConfig.name}</Badge>
                      <Badge variant="outline">Model: {agentConfig.model}</Badge>
                    </div>
                  )}
                </Card>
              </div>
            ) : (
              <div className="space-y-4">
                {messages.map((message) => (
                  <ChatMessageComponent key={message.id} message={message} />
                ))}
                {isLoading && (
                  <div className="flex items-center space-x-2 text-muted-foreground">
                    <Loader2 className="h-4 w-4 animate-spin" />
                    <span>Agent is thinking...</span>
                  </div>
                )}
                <div ref={messagesEndRef} />

                {/* Thinking steps */}
                {thinkingSteps.length > 0 && (
                  <div className="space-y-2 rounded-md border border-border bg-muted/40 p-3">
                    <div className="flex items-center justify-between text-xs font-medium text-muted-foreground">
                      <span>Thinking Steps ({thinkingSteps.length})</span>
                      <button
                        type="button"
                        className="text-[11px] underline"
                        onClick={() => setShowThinking((v) => !v)}
                      >
                        {showThinking ? '收起' : '展开'}
                      </button>
                    </div>
                    {showThinking && (
                      <div className="space-y-1">
                        {thinkingSteps.map((step, idx) => (
                          <div key={idx} className="text-xs leading-5">
                            <span className="mr-2 text-[10px] text-muted-foreground">
                              {new Date(step.timestamp).toLocaleTimeString()}
                            </span>
                            {step.content}
                          </div>
                        ))}
                      </div>
                    )}
                  </div>
                )}

                {/* Tool calls */}
                {toolCalls.length > 0 && (
                  <div className="space-y-2 rounded-md border border-border bg-muted/40 p-3">
                    <div className="flex items-center justify-between text-xs font-medium text-muted-foreground">
                      <span>Tool Calls / MCP ({toolCalls.length})</span>
                      <button
                        type="button"
                        className="text-[11px] underline"
                        onClick={() => setShowToolCalls((v) => !v)}
                      >
                        {showToolCalls ? '收起' : '展开'}
                      </button>
                    </div>
                    {showToolCalls && (
                      <div className="space-y-2">
                        {toolCalls.map((tool, idx) => (
                          <div key={tool.id || idx} className="rounded border border-border bg-background p-2 text-xs">
                            <div className="flex items-center justify-between">
                              <div className="font-semibold">{tool.name}</div>
                              <div className="text-[10px] text-muted-foreground">
                                {new Date(tool.timestamp).toLocaleTimeString()}
                              </div>
                            </div>
                            <div className="text-[11px] text-muted-foreground">Status: {tool.status}</div>
                            {tool.arguments && (
                              <div className="mt-1">
                                <div className="font-medium">Args:</div>
                                <pre className="whitespace-pre-wrap break-all rounded bg-muted p-2">
                                  {tool.arguments}
                                </pre>
                              </div>
                            )}
                            {tool.result && (
                              <div className="mt-1">
                                <div className="font-medium">Result:</div>
                                <pre className="whitespace-pre-wrap break-all rounded bg-muted p-2">
                                  {tool.result}
                                </pre>
                              </div>
                            )}
                          </div>
                        ))}
                      </div>
                    )}
                  </div>
                )}
              </div>
            )}
          </div>
        </ScrollArea>
      </div>

      {/* Input Area */}
      <div className="border-t border-border p-4">
        <div className="flex space-x-2">
          <div className="flex-1">
            <Textarea
              ref={textareaRef}
              value={input}
              onChange={(e) => setInput(e.target.value)}
              onKeyDown={handleKeyDown}
              placeholder="Type your message here..."
              className="min-h-[60px] max-h-[120px] resize-none"
              disabled={isLoading}
            />
            <div className="flex items-center justify-between mt-2 gap-4">
              <div className="flex items-center space-x-3 flex-1">
                <div className="flex items-center space-x-2">
                  <Switch
                    id="streaming-mode"
                    checked={streamingEnabled}
                    onCheckedChange={setStreamingEnabled}
                    disabled={isLoading}
                  />
                  <label
                    htmlFor="streaming-mode"
                    className="text-xs font-medium cursor-pointer"
                  >
                    Streaming
                  </label>
                </div>

                <div className="flex items-center space-x-2">
                  <Building2 className="h-3 w-3 text-muted-foreground" />
                  <Label htmlFor="org-id" className="text-xs">Org:</Label>
                  <Input
                    id="org-id"
                    type="text"
                    value={organizationId}
                    onChange={(e) => setOrganizationId(e.target.value)}
                    placeholder="default"
                    className="h-7 w-24 text-xs"
                    disabled={isLoading}
                  />
                </div>

                <div className="flex items-center space-x-2">
                  <Label htmlFor="conv-id" className="text-xs">Conv:</Label>
                  <Input
                    id="conv-id"
                    type="text"
                    value={conversationId}
                    onChange={(e) => setConversationId(e.target.value)}
                    placeholder="auto-generate"
                    className="h-7 w-32 text-xs"
                    disabled={isLoading}
                  />
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={generateNewConversationId}
                    disabled={isLoading}
                    className="h-7 w-7 p-0"
                    title="Generate new UUID"
                  >
                    <RefreshCw className="h-3 w-3" />
                  </Button>
                </div>

                <span className="text-xs text-muted-foreground ml-2">
                  {charCount} chars
                </span>
              </div>
              <div className="flex items-center space-x-2">
                <Button
                  variant="outline"
                  size="sm"
                  onClick={clearChat}
                  disabled={messages.length === 0}
                >
                  <Trash2 className="h-4 w-4" />
                </Button>
                <Button
                  onClick={sendMessage}
                  disabled={(!input.trim() && pendingImages.length === 0 && !(uploadEnabled && uploadedFile)) || isLoading}
                  size="sm"
                >
                  {isLoading ? (
                    <Loader2 className="h-4 w-4 animate-spin" />
                  ) : (
                    <Send className="h-4 w-4" />
                  )}
                </Button>
              </div>
            </div>
            <div className="flex items-start justify-between mt-3 gap-4">
              <div className="flex items-center space-x-2">
                <Switch
                  id="upload-enabled"
                  checked={uploadEnabled}
                  onCheckedChange={setUploadEnabled}
                  disabled={isLoading}
                />
                <label
                  htmlFor="upload-enabled"
                  className="text-xs font-medium cursor-pointer"
                >
                  启用文件上传
                </label>
                {uploadedFile && (
                  <span className="text-xs text-muted-foreground">
                    已上传: {uploadedFile.name}（路径: {uploadedFile.absolutePath}）
                  </span>
                )}
              </div>
              <div className="flex flex-1 items-start justify-end gap-4">
                {/* Hidden image input (triggered by button) */}
                <input
                  ref={imageInputRef}
                  id="image-upload"
                  type="file"
                  accept="image/*"
                  multiple
                  disabled={isLoading}
                  className="hidden"
                  onChange={(e) => {
                    void handleImageFiles(e.target.files);
                    // allow selecting the same file again
                    e.currentTarget.value = '';
                  }}
                />

                {/* Image attach controls */}
                <div className="flex items-center gap-2">
                  <Label htmlFor="image-upload" className="text-xs">图片:</Label>
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    className="h-8 px-2 text-xs"
                    disabled={isLoading}
                    onClick={() => imageInputRef.current?.click()}
                  >
                    选择图片
                  </Button>
                  {pendingImages.length > 0 && (
                    <>
                      <span className="text-xs text-muted-foreground">
                        已选 {pendingImages.length} 张
                      </span>
                      <Button
                        type="button"
                        variant="outline"
                        size="sm"
                        className="h-8 px-2 text-xs"
                        disabled={isLoading}
                        onClick={() => setPendingImages([])}
                        title="Clear attached images"
                      >
                        清空
                      </Button>
                    </>
                  )}
                </div>

                {/* File transfer controls */}
                <div className="flex-1">
                  <FileTransfer
                    enabled={uploadEnabled && !isLoading}
                    onUploaded={(info) => setUploadedFile(info)}
                  />
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
