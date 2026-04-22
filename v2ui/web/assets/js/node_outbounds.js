const app = new Vue({
    el: '#app',
    delimiters: ['[[', ']]'],
    data: {
        loading: false,
        searchKey: "",
        dbNodeOutbounds: [],
        nodeOutbounds: [],
        total: {
            up: 0,
            down: 0,
        },
        columns: [
            {
                title: '操作',
                key: 'action',
                scopedSlots: { customRender: 'action' },
                width: 80,
                align: 'center',
            },
            {
                title: '启用',
                key: 'enable',
                scopedSlots: { customRender: 'enable' },
                width: 80,
                align: 'center',
            },
            {
                title: 'ID',
                dataIndex: 'inboundId',
                width: 70,
                align: 'center',
            },
            {
                title: '备注',
                dataIndex: 'remark',
                ellipsis: true,
                width: 220,
            },
            {
                title: '协议',
                key: 'protocol',
                scopedSlots: { customRender: 'protocol' },
                width: 110,
                align: 'center',
            },
            {
                title: '端口',
                dataIndex: 'port',
                width: 90,
                align: 'center',
            },
            {
                title: '流量',
                key: 'traffic',
                scopedSlots: { customRender: 'traffic' },
                width: 180,
                align: 'center',
            },
            {
                title: '代理地址',
                key: 'proxyAddress',
                scopedSlots: { customRender: 'proxyAddress' },
                width: 180,
                ellipsis: true,
            },
            {
                title: '代理端口',
                key: 'proxyPort',
                scopedSlots: { customRender: 'proxyPort' },
                width: 100,
                align: 'center',
            },
            {
                title: '延迟结果',
                key: 'latencyResult',
                scopedSlots: { customRender: 'latencyResult' },
                width: 260,
                align: 'left',
            },
            {
                title: '延迟检测',
                key: 'latencyAction',
                scopedSlots: { customRender: 'latencyAction' },
                width: 130,
                align: 'center',
            },
        ],
        currentNodeOutbound: new NodeOutboundItem(),
        outboundModal: {
            visible: false,
            confirmLoading: false,
        },
    },
    created() {
        this.getNodeOutbounds();
    },
    methods: {
        async getNodeOutbounds() {
            this.loading = true;
            try {
                const msg = await HttpUtil.post('/xui/nodeOutbounds/list');
                if (!msg.success) {
                    this.$message.error(msg.msg || '获取节点出口列表失败');
                    return;
                }
                const list = (msg.obj || []).map(item => new NodeOutboundItem(item));
                this.dbNodeOutbounds = list;
                this.nodeOutbounds = list;
                this.calcTotal();
            } catch (e) {
                this.$message.error('获取节点出口列表失败');
            } finally {
                this.loading = false;
            }
        },
        searchNodeOutbounds() {
            const key = (this.searchKey || '').trim().toLowerCase();
            if (ObjectUtil.isEmpty(key)) {
                this.nodeOutbounds = this.dbNodeOutbounds;
                this.calcTotal();
                return;
            }
            this.nodeOutbounds = this.dbNodeOutbounds.filter(item => {
                return String(item.inboundId).includes(key) ||
                    String(item.port).includes(key) ||
                    (item.remark || '').toLowerCase().includes(key) ||
                    (item.protocol || '').toLowerCase().includes(key) ||
                    (item.outboundAddress || '').toLowerCase().includes(key) ||
                    String(item.outboundPort || '').includes(key);
            });
            this.calcTotal();
        },
        calcTotal() {
            let up = 0;
            let down = 0;
            this.nodeOutbounds.forEach(item => {
                up += item.up || 0;
                down += item.down || 0;
            });
            this.total.up = up;
            this.total.down = down;
        },
        resetCurrentNodeOutbound() {
            this.currentNodeOutbound = new NodeOutboundItem();
        },
        openEditNodeOutbound(row) {
            this.resetCurrentNodeOutbound();
            this.currentNodeOutbound = new NodeOutboundItem(row);
            this.outboundModal.visible = true;
        },
        closeNodeOutboundModal() {
            this.outboundModal.visible = false;
            this.outboundModal.confirmLoading = false;
            this.resetCurrentNodeOutbound();
        },
        hasValidAuthPair(row) {
            const username = (row.outboundUsername || '').trim();
            const password = (row.outboundPassword || '').trim();
            if (username === '' && password === '') {
                return true;
            }
            return username !== '' && password !== '';
        },
        isOutboundConfigValid(row) {
            return row != null &&
                !ObjectUtil.isEmpty(row.outboundAddress) &&
                Number(row.outboundPort) > 0 &&
                this.hasValidAuthPair(row);
        },
        getLatencyTagColor(status) {
            return status === '成功' ? 'green' : 'red';
        },
        async detectLatency(row) {
            if (!this.isOutboundConfigValid(row)) {
                this.$message.warning('请先配置完整的 socks5 地址和端口');
                return;
            }
            this.$set(row, 'latencyLoading', true);
            try {
                const msg = await HttpUtil.post('/xui/nodeOutbounds/latency', {
                    inboundId: row.inboundId,
                    address: row.outboundAddress,
                    port: row.outboundPort,
                });
                if (!msg.success) {
                    this.$message.error(msg.msg || '检测延迟失败');
                    return;
                }
                this.$set(row, 'latencyResult', msg.obj || null);
                this.$message.success('延迟检测完成');
            } catch (e) {
                this.$message.error('检测延迟失败');
            } finally {
                this.$set(row, 'latencyLoading', false);
            }
        },
        async saveNodeOutbound() {
            if (ObjectUtil.isEmpty(this.currentNodeOutbound.outboundAddress) || Number(this.currentNodeOutbound.outboundPort) <= 0) {
                this.$message.warning('请先填写完整的 socks5 地址和端口');
                return;
            }
            if (!this.hasValidAuthPair(this.currentNodeOutbound)) {
                this.$message.warning('用户名和密码必须同时填写或同时留空');
                return;
            }
            this.outboundModal.confirmLoading = true;
            try {
                const msg = await HttpUtil.post('/xui/nodeOutbounds/save', {
                    inboundId: this.currentNodeOutbound.inboundId,
                    enable: !!this.currentNodeOutbound.outboundEnable,
                    protocol: 'socks5',
                    address: this.currentNodeOutbound.outboundAddress,
                    port: this.currentNodeOutbound.outboundPort,
                    username: this.currentNodeOutbound.outboundUsername,
                    password: this.currentNodeOutbound.outboundPassword,
                });
                if (!msg.success) {
                    this.$message.error(msg.msg || '保存失败');
                    return;
                }
                this.$message.success('保存成功');
                this.closeNodeOutboundModal();
                await this.getNodeOutbounds();
            } catch (e) {
                this.$message.error('保存失败');
            } finally {
                this.outboundModal.confirmLoading = false;
            }
        },
        async delNodeOutbound(row) {
            this.$confirm({
                title: '确认删除该节点的出口代理配置吗？',
                content: '删除后该节点将恢复默认出口，不会删除入站节点本身。',
                okText: '确认',
                cancelText: '取消',
                onOk: async () => {
                    try {
                        const msg = await HttpUtil.post(`/xui/nodeOutbounds/del/${row.inboundId}`);
                        if (!msg.success) {
                            this.$message.error(msg.msg || '删除失败');
                            return;
                        }
                        this.$message.success('删除成功');
                        await this.getNodeOutbounds();
                    } catch (e) {
                        this.$message.error('删除失败');
                    }
                }
            });
        },
        async toggleNodeOutbound(row, checked) {
            if (checked && !this.isOutboundConfigValid(row)) {
                this.$message.warning('请先配置完整的 socks5 代理信息，再启用');
                row.outboundEnable = false;
                return;
            }
            try {
                const msg = await HttpUtil.post('/xui/nodeOutbounds/toggle', {
                    inboundId: row.inboundId,
                    enable: checked,
                });
                if (!msg.success) {
                    this.$message.error(msg.msg || '切换失败');
                    row.outboundEnable = !checked;
                    return;
                }
                row.outboundEnable = checked;
                this.$message.success(checked ? '已启用节点出口代理' : '已关闭节点出口代理');
                await this.getNodeOutbounds();
            } catch (e) {
                row.outboundEnable = !checked;
                this.$message.error('切换失败');
            }
        },
    }
});