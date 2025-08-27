#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
Token Usage Statistics Analyzer

独立的统计程序，分析特定时间段内的 API 请求数据。
从 SQLite 数据库中提取和分析 token 使用情况、rate limit 状态和GAC积分。

作者：Claude Code Companion
版本：1.1.0
"""

import sqlite3
import json
import argparse
from datetime import datetime
from typing import Dict, Tuple, Any, Optional
import sys
from collections import defaultdict


class TokenUsageAnalyzer:
    """Token 使用情况分析器"""
    
    def __init__(self, db_path: str, debug: bool = False):
        """
        初始化分析器
        
        Args:
            db_path: SQLite 数据库路径
            debug: 是否启用调试模式
        """
        self.db_path = db_path
        self.debug = debug
        self.conn = None
        self.stats = defaultdict(lambda: {
            'request_count': 0,
            'input_tokens': 0,
            'cache_creation_input_tokens': 0,
            'cache_read_input_tokens': 0,
            'output_tokens': 0,
            'unique_sessions': set()  # 用于跟踪每个状态的unique session
        })
        
    def connect_database(self) -> bool:
        """
        连接到 SQLite 数据库
        
        Returns:
            bool: 连接成功返回 True，失败返回 False
        """
        try:
            self.conn = sqlite3.connect(self.db_path)
            self.conn.row_factory = sqlite3.Row  # 允许按列名访问
            print(f"✓ 成功连接到数据库: {self.db_path}")
            return True
        except sqlite3.Error as e:
            print(f"✗ 数据库连接失败: {e}")
            return False
    
    def close_database(self):
        """关闭数据库连接"""
        if self.conn:
            self.conn.close()
            
    def build_query(self, start_time: str, end_time: str) -> str:
        """
        构建 SQL 查询语句
        
        Args:
            start_time: 开始时间 (UTC格式: YYYY-MM-DD HH:MM:SS)
            end_time: 结束时间 (UTC格式: YYYY-MM-DD HH:MM:SS)
            
        Returns:
            str: SQL 查询语句
        """
        query = """
        SELECT 
            original_response_body,
            original_response_headers,
            model,
            timestamp,
            endpoint,
            status_code,
            session_id
        FROM request_logs 
        WHERE timestamp >= ?
          AND timestamp <= ?
          AND endpoint LIKE '%api.anthropic.com%'
          AND status_code = 200
          AND (model IS NULL OR model NOT LIKE '%haiku%')
        ORDER BY timestamp
        """
        return query
        
    def parse_response_body(self, body: str) -> Dict[str, int]:
        """
        解析响应体中的 token 使用情况 (处理 stream 格式)
        
        Args:
            body: 响应体 stream 格式字符串
            
        Returns:
            Dict: 包含各种 token 数量的字典
        """
        token_usage = {
            'input_tokens': 0,
            'cache_creation_input_tokens': 0,
            'cache_read_input_tokens': 0,
            'output_tokens': 0
        }
        
        try:
            if not body or body.strip() == '':
                if self.debug:
                    print("DEBUG: 响应体为空")
                return token_usage
            
            # 处理 stream 格式响应
            lines = body.strip().split('\n')
            
            if self.debug:
                print(f"DEBUG: 找到 {len(lines)} 行响应数据")
            
            for line_num, line in enumerate(lines, 1):
                line = line.strip()
                
                # 跳过空行和非data行
                if not line:
                    continue
                if not line.startswith('data: '):
                    if self.debug and line:
                        print(f"🔍 DEBUG: 第{line_num}行跳过非data行: {line[:50]}...")
                    continue
                    
                # 提取JSON部分
                json_str = line[6:]  # 去掉 "data: " 前缀
                
                # 跳过特殊标记
                if json_str in ['[DONE]', '']:
                    if self.debug:
                        print(f"🔍 DEBUG: 第{line_num}行跳过特殊标记: {json_str}")
                    continue
                
                try:
                    data = json.loads(json_str)
                    event_type = data.get('type', 'unknown')
                    
                    if self.debug:
                        print(f"🔍 DEBUG: 第{line_num}行解析成功，事件类型: {event_type}")
                    
                    # 检查message_start事件中的usage（初始值）
                    if event_type == 'message_start':
                        message = data.get('message', {})
                        usage = message.get('usage', {})
                        
                        if usage and self.debug:
                            print(f"🔍 DEBUG: message_start usage: {usage}")
                        
                        # 使用message_start的token值作为初始值
                        for key in token_usage.keys():
                            if key in usage:
                                token_usage[key] = usage.get(key, 0)
                    
                    # 检查message_delta事件中的usage（最终完整统计）
                    elif event_type == 'message_delta':
                        # message_delta可能在delta中包含usage，也可能直接在根级别包含usage
                        usage = None
                        if 'delta' in data and 'usage' in data['delta']:
                            usage = data['delta']['usage']
                        elif 'usage' in data:
                            usage = data['usage']
                        
                        if usage and self.debug:
                            print(f"🔍 DEBUG: message_delta usage: {usage}")
                        
                        # message_delta的usage包含完整的最终统计，覆盖所有token值
                        if usage:
                            for key in token_usage.keys():
                                if key in usage:
                                    token_usage[key] = usage.get(key, 0)
                    
                    # 检查直接包含usage字段的其他事件
                    elif 'usage' in data:
                        usage = data['usage']
                        if self.debug:
                            print(f"🔍 DEBUG: 直接usage字段 (事件类型: {event_type}): {usage}")
                        # 使用最后出现的token值（覆盖，不累加）
                        for key in token_usage.keys():
                            if key in usage:
                                token_usage[key] = usage.get(key, 0)
                            
                except json.JSONDecodeError as e:
                    # 单行JSON解析失败，继续处理下一行
                    if self.debug:
                        print(f"🔍 DEBUG: 第{line_num}行JSON解析失败: {e}")
                        print(f"🔍 DEBUG: 原始内容: {json_str[:100]}...")
                    continue
                    
        except Exception as e:
            print(f"⚠ 响应体解析错误: {e}")
            
        if self.debug:
            print(f"🔍 DEBUG: 最终提取的token数量: {token_usage}")
            
        return token_usage
        
    def parse_response_headers(self, headers: str) -> str:
        """
        解析响应头中的 rate limit 状态
        
        Args:
            headers: 响应头 JSON 字符串
            
        Returns:
            str: Rate limit 状态 (allowed/allowed_warning/rejected/unknown)
        """
        try:
            if not headers or headers.strip() == '':
                return 'unknown'
                
            data = json.loads(headers)
            status = data.get('Anthropic-Ratelimit-Unified-5h-Status', 'unknown')
            return status.lower()
            
        except json.JSONDecodeError as e:
            print(f"⚠ JSON 解析失败 (response_headers): {e}")
            return 'unknown'
        except Exception as e:
            print(f"⚠ 响应头解析错误: {e}")
            return 'unknown'
            
    def analyze_data(self, start_time: str, end_time: str) -> Tuple[int, int]:
        """
        分析指定时间范围内的数据
        
        Args:
            start_time: 开始时间 (UTC)
            end_time: 结束时间 (UTC)
            
        Returns:
            Tuple[int, int]: (处理的记录总数, 解析错误的记录数)
        """
        if not self.conn:
            print("✗ 数据库未连接")
            return 0, 0
            
        query = self.build_query(start_time, end_time)
        cursor = self.conn.cursor()
        
        try:
            cursor.execute(query, (start_time, end_time))
            rows = cursor.fetchall()
            
            if not rows:
                print("⚠ 未找到符合条件的数据")
                return 0, 0
                
            total_records = len(rows)
            parse_errors = 0
            
            print(f"📊 开始分析 {total_records} 条记录...")
            
            for i, row in enumerate(rows, 1):
                if self.debug:
                    print(f"\n🔍 DEBUG: === 处理第 {i}/{total_records} 条记录 ===")
                    print(f"🔍 DEBUG: 时间戳: {row['timestamp']}")
                    print(f"🔍 DEBUG: 端点: {row['endpoint']}")
                    print(f"🔍 DEBUG: 模型: {row['model']}")
                    print(f"🔍 DEBUG: Session ID: {row['session_id']}")
                
                # 解析 token 使用情况
                token_usage = self.parse_response_body(row['original_response_body'])
                
                # 解析 rate limit 状态
                rate_limit_status = self.parse_response_headers(row['original_response_headers'])
                
                if self.debug:
                    print(f"🔍 DEBUG: Rate limit 状态: {rate_limit_status}")
                    print(f"🔍 DEBUG: Token 使用情况: {token_usage}")
                
                # 检查是否有解析错误
                if rate_limit_status == 'unknown':
                    parse_errors += 1
                
                # 累计统计
                status_stats = self.stats[rate_limit_status]
                status_stats['request_count'] += 1
                for key, value in token_usage.items():
                    status_stats[key] += value
                
                # 添加session_id到对应状态的unique session集合中
                session_id = row['session_id']
                if session_id:  # 只有当session_id不为空时才添加
                    status_stats['unique_sessions'].add(session_id)
                    
            return total_records, parse_errors
            
        except sqlite3.Error as e:
            print(f"✗ SQL 查询错误: {e}")
            return 0, 0
        finally:
            cursor.close()
            
    def format_number(self, num: int) -> str:
        """格式化数字，添加千分位分隔符"""
        return f"{num:,}"
        
    def calculate_gac_points(self, stats: Dict) -> int:
        """
        计算GAC积分
        
        公式: round((总token数 / 3072)) + (总请求数 * 2)
        总token数 = input_tokens + cache_creation_input_tokens + cache_read_input_tokens + output_tokens
        
        Args:
            stats: 包含token统计的字典
            
        Returns:
            int: GAC积分
        """
        total_tokens = (
            stats['input_tokens'] + 
            stats['cache_creation_input_tokens'] + 
            stats['cache_read_input_tokens'] + 
            stats['output_tokens']
        )
        
        token_points = round(total_tokens / 3072)
        request_points = stats['request_count'] * 2
        
        return token_points + request_points
        
    def print_results(self, start_time: str, end_time: str, 
                     total_records: int, parse_errors: int):
        """
        打印统计结果
        
        Args:
            start_time: GMT+8 开始时间
            end_time: GMT+8 结束时间  
            total_records: 处理的记录总数
            parse_errors: 解析错误数
        """
        print("\n" + "="*50)
        print("Token Usage Statistics Report")
        print("="*50)
        print(f"Time Range: {start_time} - {end_time} (GMT+8)")
        print("Filter: api.anthropic.com, Status=200, Non-Haiku models")
        print("\nSummary by Rate Limit Status:")
        print("-" * 30)
        
        # 按状态优先级排序
        status_order = ['allowed', 'allowed_warning', 'rejected', 'unknown']
        
        for status in status_order:
            if status in self.stats:
                stats = self.stats[status]
                status_display = status.upper().replace('_', '_')
                
                print(f"\n{status_display}:")
                print(f"  Request Count: {self.format_number(stats['request_count'])}")
                print(f"  Unique Sessions: {self.format_number(len(stats['unique_sessions']))}")
                
                if stats['request_count'] > 0:
                    print(f"  Total Input Tokens: {self.format_number(stats['input_tokens'])}")
                    if stats['cache_creation_input_tokens'] > 0:
                        print(f"  Total Cache Creation Tokens: {self.format_number(stats['cache_creation_input_tokens'])}")
                    if stats['cache_read_input_tokens'] > 0:
                        print(f"  Total Cache Read Tokens: {self.format_number(stats['cache_read_input_tokens'])}")
                    print(f"  Total Output Tokens: {self.format_number(stats['output_tokens'])}")
                    
                    # 计算并显示GAC积分
                    gac_points = self.calculate_gac_points(stats)
                    print(f"  GAC Points: {self.format_number(gac_points)}")
        
        # 显示其他状态（如果有）
        other_statuses = set(self.stats.keys()) - set(status_order)
        for status in sorted(other_statuses):
            stats = self.stats[status]
            print(f"\n{status.upper()}:")
            print(f"  Request Count: {self.format_number(stats['request_count'])}")
            print(f"  Unique Sessions: {self.format_number(len(stats['unique_sessions']))}")
            
            if stats['request_count'] > 0:
                # 计算并显示GAC积分
                gac_points = self.calculate_gac_points(stats)
                print(f"  GAC Points: {self.format_number(gac_points)}")
            
        print(f"\nTotal Processed Records: {self.format_number(total_records)}")
        if parse_errors > 0:
            print(f"Parse Errors: {self.format_number(parse_errors)}")
            
    def validate_time_format(self, time_str: str) -> bool:
        """
        验证时间格式
        
        Args:
            time_str: 时间字符串 (YYYY-MM-DD HH:MM:SS)
            
        Returns:
            bool: 格式正确返回 True
        """
        try:
            datetime.strptime(time_str, '%Y-%m-%d %H:%M:%S')
            return True
        except ValueError as e:
            print(f"✗ 时间格式错误: {e}")
            return False
            
    def run_analysis(self, start_time: str, end_time: str) -> bool:
        """
        运行完整的分析流程
        
        Args:
            start_time: 开始时间 (GMT+8格式: YYYY-MM-DD HH:MM:SS)
            end_time: 结束时间 (GMT+8格式: YYYY-MM-DD HH:MM:SS)
            
        Returns:
            bool: 分析成功返回 True
        """
        print(f"🔍 开始分析时间范围: {start_time} - {end_time} (GMT+8)")
        
        # 验证时间格式
        if not self.validate_time_format(start_time) or not self.validate_time_format(end_time):
            return False
        
        # 连接数据库
        if not self.connect_database():
            return False
            
        try:
            # 直接使用GMT+8时间进行分析（不进行时区转换）
            total_records, parse_errors = self.analyze_data(start_time, end_time)
            
            if total_records == 0:
                print("⚠ 没有找到符合条件的数据记录")
                return False
                
            # 打印结果
            self.print_results(start_time, end_time, total_records, parse_errors)
            
            return True
            
        finally:
            self.close_database()


def main():
    """主程序入口"""
    parser = argparse.ArgumentParser(
        description='Token Usage Statistics Analyzer',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog='''
示例用法:
  %(prog)s --db ./logs/logs.db
  %(prog)s --db /path/to/logs.db --start "2025-08-26 10:00:00" --end "2025-08-26 20:00:00"
  %(prog)s --db ./logs/logs.db --debug  # 启用调试模式
  %(prog)s --help
        '''
    )
    
    parser.add_argument(
        '--db', 
        default='./logs/logs.db',
        help='SQLite 数据库文件路径 (默认: ./logs/logs.db)'
    )
    
    parser.add_argument(
        '--start',
        default='2025-08-26 14:00:00',
        help='开始时间 GMT+8 (格式: YYYY-MM-DD HH:MM:SS, 默认: 2025-08-26 14:00:00)'
    )
    
    parser.add_argument(
        '--end',
        default='2025-08-26 18:00:00', 
        help='结束时间 GMT+8 (格式: YYYY-MM-DD HH:MM:SS, 默认: 2025-08-26 18:00:00)'
    )
    
    parser.add_argument(
        '--debug',
        action='store_true',
        help='启用调试模式，显示详细的解析过程'
    )
    
    args = parser.parse_args()
    
    print("Token Usage Statistics Analyzer v1.1.0")
    print("-" * 40)
    
    if args.debug:
        print("🔍 调试模式已启用")
    
    # 创建分析器实例
    analyzer = TokenUsageAnalyzer(args.db, debug=args.debug)
    
    # 运行分析
    success = analyzer.run_analysis(args.start, args.end)
    
    if not success:
        sys.exit(1)
        
    print("\n✓ 分析完成!")


if __name__ == '__main__':
    main()