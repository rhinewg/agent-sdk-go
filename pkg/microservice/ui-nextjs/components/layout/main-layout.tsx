'use client';

import React, { useState, useEffect } from 'react';
import { ChatArea } from '../chat/chat-area';
import { AgentInfoScreen } from '../screens/agent-info-screen';
import { ToolsScreen } from '../screens/tools-screen';
import { MemoryScreen } from '../screens/memory-screen';
import { SubAgentsScreen } from '../screens/sub-agents-screen';
import { SettingsScreen } from '../screens/settings-screen';
import { TracesScreen } from '../screens/traces-screen';
import { AgentConfig } from '@/types/agent';
import { agentAPI } from '@/lib/api';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import {
  MessageSquare,
  Bot,
  Wrench,
  Database,
  Users,
  Settings,
  Activity,
  LogIn,
  LogOut,
} from 'lucide-react';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';

type ActiveScreen = 'chat' | 'agent-info' | 'tools' | 'memory' | 'sub-agents' | 'traces' | 'settings';

interface NavigationItem {
  id: ActiveScreen;
  label: string;
  icon: React.ComponentType<{ className?: string }>;
  description: string;
}

const navigationItems: NavigationItem[] = [
  {
    id: 'chat',
    label: 'Chat',
    icon: MessageSquare,
    description: 'Chat with the agent'
  },
  {
    id: 'agent-info',
    label: 'Agent Info',
    icon: Bot,
    description: 'View agent configuration and details'
  },
  {
    id: 'tools',
    label: 'Tools',
    icon: Wrench,
    description: 'Available tools and capabilities'
  },
  {
    id: 'memory',
    label: 'Memory',
    icon: Database,
    description: 'Browse conversation history and traces'
  },
  {
    id: 'sub-agents',
    label: 'Sub-Agents',
    icon: Users,
    description: 'Manage and delegate tasks to sub-agents'
  },
  {
    id: 'traces',
    label: 'Traces',
    icon: Activity,
    description: 'Monitor agent execution traces and performance'
  },
  {
    id: 'settings',
    label: 'Settings',
    icon: Settings,
    description: 'Application settings and preferences'
  }
];

export function MainLayout() {
  const [activeScreen, setActiveScreen] = useState<ActiveScreen>('chat');
  const [agentConfig, setAgentConfig] = useState<AgentConfig | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [isAuthenticated, setIsAuthenticated] = useState(false);
  const [loginOpen, setLoginOpen] = useState(false);
  const [loginLoading, setLoginLoading] = useState(false);
  const [loginError, setLoginError] = useState<string | null>(null);
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');

  useEffect(() => {
    // 尝试从 localStorage 恢复已有 token
    if (typeof window !== 'undefined') {
      const savedToken = window.localStorage.getItem('agent_auth_token');
      if (savedToken) {
        agentAPI.setAuthToken(savedToken);
        setIsAuthenticated(true);
      }
    }
    loadAgentConfig(false);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // System theme detection when agent config is loaded
  useEffect(() => {
    if (agentConfig?.ui_theme === 'system') {
      const applySystemTheme = () => {
        const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
        if (prefersDark) {
          document.documentElement.classList.add('dark');
        } else {
          document.documentElement.classList.remove('dark');
        }
      };

      // Apply initial theme
      applySystemTheme();

      // Listen for system theme changes
      const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)');
      const handleChange = () => applySystemTheme();

      mediaQuery.addEventListener('change', handleChange);
      return () => mediaQuery.removeEventListener('change', handleChange);
    }
  }, [agentConfig?.ui_theme]);

  const loadAgentConfig = async (openLoginOnUnauthorized = false) => {
    try {
      setLoading(true);
      setError(null);
      const config = await agentAPI.getAgentConfig();
      setAgentConfig(config);
      setIsAuthenticated(true);
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Failed to load agent config';
      // 未认证时后端会返回 401，这里视为“未登录”，弹出登录框而不是报错页
      if (msg.includes('401')) {
        setIsAuthenticated(false);
        setAgentConfig(null);
        setError(null);
        if (openLoginOnUnauthorized) {
          setLoginOpen(true);
        }
      } else {
        setError(msg);
      }
    } finally {
      setLoading(false);
    }
  };

  const handleLogin = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoginLoading(true);
    setLoginError(null);
    try {
      const result = await agentAPI.login(username.trim(), password);
      if (typeof window !== 'undefined') {
        window.localStorage.setItem('agent_auth_token', result.token);
      }
      setIsAuthenticated(true);
      setLoginOpen(false);
      setUsername('');
      setPassword('');
      await loadAgentConfig();
    } catch (err) {
      setLoginError(err instanceof Error ? err.message : '登录失败');
    } finally {
      setLoginLoading(false);
    }
  };

  const handleLogout = () => {
    agentAPI.setAuthToken(null);
    if (typeof window !== 'undefined') {
      window.localStorage.removeItem('agent_auth_token');
    }
    setIsAuthenticated(false);
    setAgentConfig(null);
    setActiveScreen('chat');
    setLoginOpen(true);
  };

  if (loading) {
    return (
      <div className="flex h-screen items-center justify-center">
        <div className="text-center">
          <div className="animate-spin rounded-full h-32 w-32 border-b-2 border-gray-900"></div>
          <p className="mt-4 text-lg">Loading Agent UI...</p>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex h-screen items-center justify-center">
        <div className="text-center">
          <p className="text-red-500 text-lg mb-4">Error: {error}</p>
          <Button onClick={() => loadAgentConfig(true)}>Retry</Button>
        </div>
      </div>
    );
  }

  return (
    <div className="h-full bg-background flex flex-col fixed inset-0">
      {/* Header */}
      <header className="h-16 border-b border-border bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/60 flex-shrink-0">
        <div className="flex items-center justify-between h-full px-4">
          <div className="flex items-center gap-4">
            <div className="flex items-center gap-2">
              <Bot className="h-6 w-6" />
              <h1 className="text-xl font-semibold">
                {agentConfig?.name || 'Agent'}
              </h1>
            </div>
            {agentConfig && (
              <Badge variant="secondary" className="text-xs">
                {agentConfig.model}
              </Badge>
            )}
          </div>

          <div className="flex items-center space-x-2">
            <div className="flex items-center space-x-1">
              <div className="h-2 w-2 bg-green-500 rounded-full"></div>
              <span className="text-sm text-muted-foreground">
                {isAuthenticated ? 'Ready' : 'Not logged in'}
              </span>
            </div>
            {isAuthenticated ? (
              <Button variant="outline" size="sm" onClick={handleLogout}>
                <LogOut className="h-4 w-4 mr-1" />
                退出登录
              </Button>
            ) : (
              <Button variant="default" size="sm" onClick={() => setLoginOpen(true)}>
                <LogIn className="h-4 w-4 mr-1" />
                登录
              </Button>
            )}
          </div>
        </div>
      </header>

      {/* Navigation */}
      <nav className="border-b border-border bg-background px-4">
        <div className="flex space-x-1 py-2">
          {navigationItems
            .filter((item) => {
              // Only show traces tab if the feature is enabled
              if (item.id === 'traces') {
                return agentConfig?.features?.traces === true;
              }
              return true;
            })
            .map((item) => {
            const Icon = item.icon;
            const isActive = activeScreen === item.id;

            return (
              <Button
                key={item.id}
                variant={isActive ? 'default' : 'ghost'}
                size="sm"
                onClick={() => setActiveScreen(item.id)}
                className={`flex items-center gap-2 ${isActive ? '' : 'text-muted-foreground'}`}
                title={item.description}
                disabled={!isAuthenticated}
              >
                <Icon className="h-4 w-4" />
                <span className="hidden sm:inline">{item.label}</span>
              </Button>
            );
          })}
        </div>
      </nav>

      {/* Main Content */}
      <main className="flex-1 overflow-auto">
        <div className="h-full overflow-y-auto">
          {isAuthenticated ? (
            <div className="h-full">
              <div className={activeScreen === 'chat' ? 'h-full' : 'hidden'}>
                <ChatArea agentConfig={agentConfig} />
              </div>
              <div className={activeScreen === 'agent-info' ? 'h-full' : 'hidden'}>
                <AgentInfoScreen agentConfig={agentConfig} />
              </div>
              <div className={activeScreen === 'tools' ? 'h-full' : 'hidden'}>
                <ToolsScreen />
              </div>
              <div className={activeScreen === 'memory' ? 'h-full' : 'hidden'}>
                <MemoryScreen />
              </div>
              <div className={activeScreen === 'sub-agents' ? 'h-full' : 'hidden'}>
                <SubAgentsScreen />
              </div>
              <div className={activeScreen === 'traces' ? 'h-full' : 'hidden'}>
                <TracesScreen />
              </div>
              <div className={activeScreen === 'settings' ? 'h-full' : 'hidden'}>
                <SettingsScreen agentConfig={agentConfig} />
              </div>
            </div>
          ) : (
            <div className="flex h-full items-center justify-center">
              <div className="text-center space-y-2">
                <p className="text-lg font-medium">请先登录以使用 Agent UI 功能</p>
                <p className="text-sm text-muted-foreground">
                  点击右上角「登录」按钮。
                </p>
                <Button variant="default" size="sm" onClick={() => setLoginOpen(true)}>
                  <LogIn className="h-4 w-4 mr-1" />
                  去登录
                </Button>
              </div>
            </div>
          )}
        </div>
      </main>

      {/* 登录弹窗 */}
      <Dialog open={loginOpen} onOpenChange={setLoginOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>登录</DialogTitle>
          </DialogHeader>
          <form className="space-y-4" onSubmit={handleLogin}>
            <div className="space-y-2">
              <Label htmlFor="login-username">用户名</Label>
              <Input
                id="login-username"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                autoComplete="username"
                required
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="login-password">密码</Label>
              <Input
                id="login-password"
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                autoComplete="current-password"
                required
              />
            </div>
            {loginError && (
              <p className="text-sm text-red-500">
                {loginError}
              </p>
            )}
            <DialogFooter>
              <Button type="submit" disabled={loginLoading}>
                {loginLoading ? '登录中...' : '登录'}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </div>
  );
}
