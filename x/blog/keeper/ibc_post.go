package keeper

import (
	"errors"

	"planet/x/blog/types"
	"strconv"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	clienttypes "github.com/cosmos/ibc-go/v6/modules/core/02-client/types"
	channeltypes "github.com/cosmos/ibc-go/v6/modules/core/04-channel/types"
	host "github.com/cosmos/ibc-go/v6/modules/core/24-host"
)

// TransmitIbcPostPacket transmits the packet over IBC with the specified source port and source channel
func (k Keeper) TransmitIbcPostPacket(
	ctx sdk.Context,
	packetData types.IbcPostPacketData,
	sourcePort,
	sourceChannel string,
	timeoutHeight clienttypes.Height,
	timeoutTimestamp uint64,
) (uint64, error) {
	channelCap, ok := k.scopedKeeper.GetCapability(ctx, host.ChannelCapabilityPath(sourcePort, sourceChannel))
	if !ok {
		return 0, sdkerrors.Wrap(channeltypes.ErrChannelCapabilityNotFound, "module does not own channel capability")
	}

	packetBytes, err := packetData.GetBytes()
	if err != nil {
		return 0, sdkerrors.Wrapf(sdkerrors.ErrJSONMarshal, "cannot marshal the packet: %w", err)
	}

	return k.channelKeeper.SendPacket(ctx, channelCap, sourcePort, sourceChannel, timeoutHeight, timeoutTimestamp, packetBytes)
}

func (k Keeper) OnRecvIbcPostPacket(ctx sdk.Context, packet channeltypes.Packet, data types.IbcPostPacketData) (packetAck types.IbcPostPacketAck, err error) {
	// validate packet data upon receiving
	if err := data.ValidateBasic(); err != nil {
		return packetAck, err
	}

	id := k.AppendPost(
		ctx,
		types.Post{
			Creator: packet.SourcePort + "-" + packet.SourceChannel + "-" + data.Creator,
			Title:   data.Title,
			Content: data.Content,
		},
	)

	packetAck.PostID = strconv.FormatUint(id, 10)

	return packetAck, nil
}

// OnAcknowledgementIbcPostPacket responds to the the success or failure of a packet
// acknowledgement written on the receiving chain.
// x/blog/keeper/ibc_post.go
func (k Keeper) OnAcknowledgementIbcPostPacket(ctx sdk.Context, packet channeltypes.Packet, data types.IbcPostPacketData, ack channeltypes.Acknowledgement) error {
	switch dispatchedAck := ack.Response.(type) {
	case *channeltypes.Acknowledgement_Error:
		// We will not treat acknowledgment error in this tutorial
		return nil
	case *channeltypes.Acknowledgement_Result:
		// Decode the packet acknowledgment
		var packetAck types.IbcPostPacketAck

		if err := types.ModuleCdc.UnmarshalJSON(dispatchedAck.Result, &packetAck); err != nil {
			// The counter-party module doesn't implement the correct acknowledgment format
			return errors.New("cannot unmarshal acknowledgment")
		}

		k.AppendSentPost(
			ctx,
			types.SentPost{
				Creator: data.Creator,
				PostID:  packetAck.PostID,
				Title:   data.Title,
				Chain:   packet.DestinationPort + "-" + packet.DestinationChannel,
			},
		)

		return nil
	default:
		return errors.New("the counter-party module does not implement the correct acknowledgment format")
	}
}

// OnTimeoutIbcPostPacket responds to the case where a packet has not been transmitted because of a timeout
func (k Keeper) OnTimeoutIbcPostPacket(ctx sdk.Context, packet channeltypes.Packet, data types.IbcPostPacketData) error {
	k.AppendTimedoutPost(
		ctx,
		types.TimedoutPost{
			Creator: data.Creator,
			Title:   data.Title,
			Chain:   packet.DestinationPort + "-" + packet.DestinationChannel,
		},
	)

	return nil
}
