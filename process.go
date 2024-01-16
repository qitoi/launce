/*
 *  Copyright 2024 qitoi
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 */

package launce

import (
	"math"
	"sync/atomic"

	"github.com/shirou/gopsutil/process"
)

type ProcessInfo struct {
	proc        *process.Process
	cpuUsage    uint64
	memoryUsage uint64
}

func NewProcessInfo(pid int) (*ProcessInfo, error) {
	proc, err := process.NewProcess(int32(pid))
	if err != nil {
		return nil, err
	}
	return &ProcessInfo{
		proc:        proc,
		cpuUsage:    0,
		memoryUsage: 0,
	}, nil
}

func (p *ProcessInfo) Update() error {
	mem, err := p.proc.MemoryInfo()
	if err != nil {
		return err
	}
	cpu, err := p.proc.CPUPercent()
	if err != nil {
		return err
	}

	atomic.StoreUint64(&p.cpuUsage, math.Float64bits(cpu))
	atomic.StoreUint64(&p.memoryUsage, mem.RSS)

	return nil
}

func (p *ProcessInfo) CPUUsage() float64 {
	return math.Float64frombits(atomic.LoadUint64(&p.cpuUsage))
}

func (p *ProcessInfo) MemoryUsage() uint64 {
	return atomic.LoadUint64(&p.memoryUsage)
}
